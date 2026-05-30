package handler

import (
	"net/http"

	"backend/response"
	"backend/service"
	"github.com/labstack/echo/v5"
)

type QuotaHandler struct {
	svc service.QuotaService
}

func NewQuotaHandler(svc service.QuotaService) *QuotaHandler {
	return &QuotaHandler{svc: svc}
}

// GetMyQuota は認証ユーザー自身のクォータ情報（使用量と上限）を返します。
func (h *QuotaHandler) GetMyQuota(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	info, err := h.svc.GetQuota(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{
		"usage":  info.Usage,
		"limits": info.Limits,
	})
}

// SetQuota は管理者がユーザーのクォータを設定します。
func (h *QuotaHandler) SetQuota(c *echo.Context) error {
	targetUserID := c.Param("user_id")
	if targetUserID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"data":  nil,
			"error": map[string]string{"code": "BAD_REQUEST", "message": "user_id is required"},
		})
	}

	var req struct {
		MaxSmall     int `json:"max_small"`
		MaxMedium    int `json:"max_medium"`
		MaxLarge     int `json:"max_large"`
		MaxStorageMB int `json:"max_storage_mb"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}
	if req.MaxSmall < 0 || req.MaxMedium < 0 || req.MaxLarge < 0 || req.MaxStorageMB < 0 {
		return badRequest(c, "quota values must be non-negative")
	}

	if err := h.svc.SetQuota(c.Request().Context(), targetUserID, req.MaxSmall, req.MaxMedium, req.MaxLarge, req.MaxStorageMB); err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{
		"user_id":        targetUserID,
		"max_small":      req.MaxSmall,
		"max_medium":     req.MaxMedium,
		"max_large":      req.MaxLarge,
		"max_storage_mb": req.MaxStorageMB,
	})
}
