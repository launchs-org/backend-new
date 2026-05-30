// Package errors は Launchs-org で使うカスタムエラー型を定義します。
// handler 層でこれらの型をチェックして適切な HTTP ステータスに変換します。
package errors

import "fmt"

// NotFoundError はリソースが存在しないときに返します（HTTP 404）。
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// ConflictError はリソースが既に存在するときに返します（HTTP 409）。
type ConflictError struct {
	Resource string
	ID       string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s already exists: %s", e.Resource, e.ID)
}

// ForbiddenError は操作権限がないときに返します（HTTP 403）。
type ForbiddenError struct {
	Message string
}

func (e *ForbiddenError) Error() string {
	return e.Message
}

// QueueFullError はワークフローキューが上限に達したときに返します（HTTP 429）。
type QueueFullError struct {
	ContainerID string
}

func (e *QueueFullError) Error() string {
	return fmt.Sprintf("workflow queue is full for container: %s", e.ContainerID)
}

// ValidationError はリクエストの入力値が不正なときに返します（HTTP 400）。
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// QuotaExceededError はユーザーのリソースクォータ上限を超えたときに返します（HTTP 429）。
type QuotaExceededError struct {
	ResourceSize string
	Current      int
	Max          int
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("quota exceeded for resource size %s: current=%d, max=%d", e.ResourceSize, e.Current, e.Max)
}

// StorageQuotaExceededError はユーザーのストレージクォータ上限を超えたときに返します（HTTP 429）。
type StorageQuotaExceededError struct {
	UsedMB      int
	RequestedMB int
	MaxMB       int
}

func (e *StorageQuotaExceededError) Error() string {
	return fmt.Sprintf("storage quota exceeded: used=%dMB, requested=%dMB, max=%dMB", e.UsedMB, e.RequestedMB, e.MaxMB)
}
