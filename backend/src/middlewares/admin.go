package middlewares

import (
	"net/http"

	"launchs/shared/config"

	"github.com/labstack/echo/v5"
)

// RequireAdminKey は X-Admin-Key ヘッダーが設定された ADMIN_API_KEY と一致するか確認します。
// ADMIN_API_KEY が空の場合はすべてのリクエストを拒否します。
func RequireAdminKey(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		key := config.AdminAPIKey()
		if key == "" {
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"data":  nil,
				"error": map[string]string{"code": "FORBIDDEN", "message": "admin API is not configured"},
			})
		}
		provided := c.Request().Header.Get("X-Admin-Key")
		if provided != key {
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"data":  nil,
				"error": map[string]string{"code": "FORBIDDEN", "message": "invalid admin key"},
			})
		}
		return next(c)
	}
}
