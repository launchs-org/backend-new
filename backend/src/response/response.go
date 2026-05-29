// Package response は API レスポンスの統一フォーマットを提供します。
// 全レスポンスは { "data": {...}, "error": null } または { "data": null, "error": {...} } の形式です。
package response

import (
	"fmt"
	"net/http"
	"runtime/debug"

	apperrors "launchs/shared/errors"

	"github.com/labstack/echo/v5"
)

// ErrorBody はエラーレスポンスのボディです。
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// APIResponse は統一レスポンス構造体です。
type APIResponse struct {
	Data  interface{} `json:"data"`
	Error interface{} `json:"error"`
}

// OK は 200 成功レスポンスを返します。
func OK(c *echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, APIResponse{Data: data, Error: nil})
}

// Created は 201 成功レスポンスを返します。
func Created(c *echo.Context, data interface{}) error {
	return c.JSON(http.StatusCreated, APIResponse{Data: data, Error: nil})
}

// Error はエラーの種類に応じた HTTP ステータスでエラーレスポンスを返します。
func Error(c *echo.Context, err error) error {
	req := c.Request()
	switch e := err.(type) {
	case *apperrors.NotFoundError:
		return c.JSON(http.StatusNotFound, APIResponse{Data: nil, Error: ErrorBody{Code: "NOT_FOUND", Message: e.Error()}})
	case *apperrors.ConflictError:
		return c.JSON(http.StatusConflict, APIResponse{Data: nil, Error: ErrorBody{Code: "CONFLICT", Message: e.Error()}})
	case *apperrors.ForbiddenError:
		return c.JSON(http.StatusForbidden, APIResponse{Data: nil, Error: ErrorBody{Code: "FORBIDDEN", Message: e.Error()}})
	case *apperrors.QueueFullError:
		return c.JSON(http.StatusTooManyRequests, APIResponse{Data: nil, Error: ErrorBody{Code: "QUEUE_FULL", Message: e.Error()}})
	case *apperrors.ValidationError:
		return c.JSON(http.StatusBadRequest, APIResponse{Data: nil, Error: ErrorBody{Code: "BAD_REQUEST", Message: e.Error()}})
	case *apperrors.QuotaExceededError:
		return c.JSON(http.StatusTooManyRequests, APIResponse{Data: nil, Error: ErrorBody{Code: "QUOTA_EXCEEDED", Message: e.Error()}})
	default:
		fmt.Printf("[ERROR] %s %s => %v\n%s\n", req.Method, req.URL.Path, err, debug.Stack())
		return c.JSON(http.StatusInternalServerError, APIResponse{Data: nil, Error: ErrorBody{Code: "INTERNAL_ERROR", Message: "internal server error"}})
	}
}
