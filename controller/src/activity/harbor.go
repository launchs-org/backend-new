package activity

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"launchs/shared/config"
)

// HarborActivity は Harbor プロジェクト・ロボットアカウント・イメージの管理を担当します。
type HarborActivity struct{}

// RobotAccountResult はロボットアカウント作成結果です。
// Temporal は複数戻り値をサポートしないため struct に包みます。
type RobotAccountResult struct {
	Username string
	Password string
}

// HarborCreateProject は Harbor にプロジェクトを作成します。
func (a *HarborActivity) HarborCreateProject(ctx context.Context, projectName string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"project_name": projectName,
		"public":       true,
	})
	resp, err := harborRequest(ctx, http.MethodPost, "/api/v2.0/projects", body)
	if err != nil {
		return fmt.Errorf("Harbor プロジェクト作成エラー: %w", err)
	}
	defer resp.Body.Close()

	// 409 は既に存在する場合なので無視
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Harbor プロジェクト作成失敗 status=%d body=%s", resp.StatusCode, string(b))
	}
	return nil
}

// HarborCreateRobotAccount は Harbor プロジェクトにロボットアカウントを作成し、認証情報を返します。
func (a *HarborActivity) HarborCreateRobotAccount(ctx context.Context, projectName string) (*RobotAccountResult, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"name":        "launchs-builder",
		"description": "Launchs builder robot account",
		"duration":    -1, // 期限なし
		"level":       "project",
		"permissions": []map[string]interface{}{
			{
				"kind":      "project",
				"namespace": projectName,
				"access": []map[string]string{
					{"resource": "repository", "action": "push"},
					{"resource": "repository", "action": "pull"},
					{"resource": "artifact", "action": "delete"},
				},
			},
		},
	})

	resp, err := harborRequest(ctx, http.MethodPost, "/api/v2.0/robots", body)
	if err != nil {
		return nil, fmt.Errorf("Harbor ロボットアカウント作成エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Harbor ロボットアカウント作成失敗 status=%d body=%s", resp.StatusCode, string(b))
	}

	var result struct {
		Name   string `json:"name"`
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("Harbor ロボットアカウントレスポンスパースエラー: %w", err)
	}
	return &RobotAccountResult{Username: result.Name, Password: result.Secret}, nil
}

// HarborDeleteProject は Harbor プロジェクトを削除します。
func (a *HarborActivity) HarborDeleteProject(ctx context.Context, projectName string) error {
	resp, err := harborRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v2.0/projects/%s", projectName), nil)
	if err != nil {
		return fmt.Errorf("Harbor プロジェクト削除エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Harbor プロジェクト削除失敗 status=%d body=%s", resp.StatusCode, string(b))
	}
	return nil
}

// HarborDeleteImage は Harbor から特定のイメージを削除します。
func (a *HarborActivity) HarborDeleteImage(ctx context.Context, projectName, repoName, tag string) error {
	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s/artifacts/%s", projectName, repoName, tag)
	resp, err := harborRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("Harbor イメージ削除エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Harbor イメージ削除失敗 status=%d body=%s", resp.StatusCode, string(b))
	}
	return nil
}

// harborRequest は Harbor API に Basic 認証付きでリクエストを送ります。
func harborRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	endpoint := config.HarborEndpoint()
	url := endpoint + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	// ログにユーザー名とパスワードを表示
	fmt.Printf("Username: %s, Password: %s", config.HarborAdminUser(), config.HarborAdminPassword())

	req.SetBasicAuth(config.HarborAdminUser(), config.HarborAdminPassword())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: transport,
	}

	return client.Do(req)
}
