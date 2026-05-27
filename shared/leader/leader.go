// Package leader は GORM + PostgreSQL を用いたシングルロウ型のリーダーエレクションを実装します。
// watcher が複数 Pod にスケールされた場合でも、リーダーに選出された Pod のみが
// Kubernetes Watch を実行することで、二重処理を防ぎます。
package leader

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const singletonID = 1

// watcherLeader は watcher_leaders テーブルのレコードです（パッケージ内部専用）。
type watcherLeader struct {
	ID        int       `gorm:"primaryKey"`
	PodID     string    `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null;index"`
	UpdatedAt time.Time
}

// EnsureTable は watcher_leaders テーブルを AutoMigrate で作成します。
func EnsureTable(db *gorm.DB) error {
	return db.AutoMigrate(&watcherLeader{})
}

// Elector はリーダーエレクションを管理します。
type Elector struct {
	db            *gorm.DB
	podID         string
	leaseTTL      time.Duration
	renewInterval time.Duration
}

// New は Elector を作成します。
//
//   - podID        : このインスタンスを識別する一意な文字列（例: POD_NAME 環境変数）
//   - leaseTTL     : リーダーシップの有効期間。この時間内にハートビートがなければ失効します。
//   - renewInterval: ハートビート送信間隔（leaseTTL の 1/3 程度が推奨）
func New(db *gorm.DB, podID string, leaseTTL, renewInterval time.Duration) *Elector {
	return &Elector{
		db:            db,
		podID:         podID,
		leaseTTL:      leaseTTL,
		renewInterval: renewInterval,
	}
}

// Run はリーダーエレクションループを開始し、このインスタンスがリーダーである間だけ
// runFn を実行します。runFn には派生 context が渡され、リーダーシップを失った際に
// キャンセルされます。ctx が Done になるとループを終了します。
func (e *Elector) Run(ctx context.Context, runFn func(ctx context.Context) error) {
	fmt.Printf("[leader] %s: リーダーエレクション開始（lease_ttl=%s, renew_interval=%s）\n",
		e.podID, e.leaseTTL, e.renewInterval)

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[leader] %s: コンテキストキャンセル、エレクションループ終了\n", e.podID)
			return
		default:
		}

		fmt.Printf("[leader] %s: リーダー取得を試みています...\n", e.podID)
		if e.tryAcquire(ctx) {
			fmt.Printf("[leader] %s: ★ リーダーに昇格しました ★ collectors を起動します\n", e.podID)
			e.runAsLeader(ctx, runFn)
			fmt.Printf("[leader] %s: リーダーシップを終了しました。再選出待ちへ移行します\n", e.podID)
		} else {
			// 現在のリーダー情報をログに出す
			e.logCurrentLeader(ctx)
			fmt.Printf("[leader] %s: スタンバイ中（%s 後に再試行）\n", e.podID, e.renewInterval)
			select {
			case <-ctx.Done():
				fmt.Printf("[leader] %s: コンテキストキャンセル、スタンバイ終了\n", e.podID)
				return
			case <-time.After(e.renewInterval):
			}
		}
	}
}

// tryAcquire はリーダーシップの取得を試みます。
// 既存レコードが存在しない、または TTL 切れの場合に自 podID で上書きして true を返します。
func (e *Elector) tryAcquire(ctx context.Context) bool {
	now := time.Now()
	newExpiry := now.Add(e.leaseTTL)

	// UPSERT: id=1 のレコードを INSERT し、競合時は expires_at が過去の場合のみ上書き。
	// PostgreSQL の ON CONFLICT DO UPDATE + WHERE 句でアトミックに制御します。
	result := e.db.WithContext(ctx).Exec(`
		INSERT INTO watcher_leaders (id, pod_id, expires_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE
		  SET pod_id = EXCLUDED.pod_id,
		      expires_at = EXCLUDED.expires_at,
		      updated_at = EXCLUDED.updated_at
		WHERE watcher_leaders.expires_at < ? OR watcher_leaders.pod_id = ?
	`, singletonID, e.podID, newExpiry, now, now, e.podID)

	if result.Error != nil {
		fmt.Printf("[leader] %s: tryAcquire DB エラー: %v\n", e.podID, result.Error)
		return false
	}
	// RowsAffected == 0 は別 Pod がリーダーであることを意味します
	if result.RowsAffected == 0 {
		return false
	}

	// 取得後、本当に自分が書き込まれたか確認（念のため）
	var rec watcherLeader
	if err := e.db.WithContext(ctx).First(&rec, singletonID).Error; err != nil {
		fmt.Printf("[leader] %s: 取得後の確認クエリエラー: %v\n", e.podID, err)
		return false
	}
	if rec.PodID != e.podID {
		fmt.Printf("[leader] %s: リーダー確認失敗（現在のリーダー: %s）\n", e.podID, rec.PodID)
		return false
	}
	return true
}

// runAsLeader はリーダーとして runFn を実行しつつ、定期的にハートビートを送ります。
// ハートビート更新に失敗した場合（他 Pod に奪取された等）は runFn の context をキャンセルします。
func (e *Elector) runAsLeader(ctx context.Context, runFn func(ctx context.Context) error) {
	leaderCtx, leaderCancel := context.WithCancel(ctx)
	defer leaderCancel()

	done := make(chan error, 1)
	go func() {
		done <- runFn(leaderCtx)
	}()

	ticker := time.NewTicker(e.renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[leader] %s: シャットダウンシグナル受信、collectors を停止します\n", e.podID)
			leaderCancel()
			<-done
			return

		case err := <-done:
			if err != nil {
				fmt.Printf("[leader] %s: collectors が終了しました（エラー: %v）\n", e.podID, err)
			} else {
				fmt.Printf("[leader] %s: collectors が正常終了しました\n", e.podID)
			}
			return

		case <-ticker.C:
			if !e.renew(ctx) {
				fmt.Printf("[leader] %s: ✗ ハートビート更新失敗 — リーダーシップを喪失しました。collectors を停止します\n", e.podID)
				leaderCancel()
				<-done
				return
			}
			fmt.Printf("[leader] %s: ハートビート更新 OK（次の失効: %s）\n",
				e.podID, time.Now().Add(e.leaseTTL).Format(time.RFC3339))
		}
	}
}

// renew はリーダーとして自分の expires_at を更新します。
// 自分が現在のリーダーでなければ false を返します。
func (e *Elector) renew(ctx context.Context) bool {
	newExpiry := time.Now().Add(e.leaseTTL)
	result := e.db.WithContext(ctx).Model(&watcherLeader{}).
		Where("id = ? AND pod_id = ?", singletonID, e.podID).
		Updates(map[string]interface{}{
			"expires_at": newExpiry,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		fmt.Printf("[leader] %s: renew DB エラー: %v\n", e.podID, result.Error)
		return false
	}
	return result.RowsAffected > 0
}

// logCurrentLeader は現在のリーダー情報をログに出力します（スタンバイ Pod 用）。
func (e *Elector) logCurrentLeader(ctx context.Context) {
	var rec watcherLeader
	if err := e.db.WithContext(ctx).First(&rec, singletonID).Error; err != nil {
		return
	}
	fmt.Printf("[leader] %s: 現在のリーダー = %s（失効: %s）\n",
		e.podID, rec.PodID, rec.ExpiresAt.Format(time.RFC3339))
}
