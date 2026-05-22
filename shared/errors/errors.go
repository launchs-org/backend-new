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
