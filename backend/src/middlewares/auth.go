package middlewares

import (
	"launchs/shared/logger"
	"net/http"

	"github.com/labstack/echo/v5"
)

// 認証ミドルウェア
func RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		// ヘッダからトークンを取得、なければ Sec-WebSocket-Protocol から取得、なければクエリパラメータから取得
		token := c.Request().Header.Get("Authorization")
		if token == "" {
			token = c.Request().Header.Get("Sec-WebSocket-Protocol")
		}
		if token == "" {
			token = c.QueryParam("token")
		}
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		// トークンを検証
		claim, err := ValidateToken(token)

		// エラー処理
		if err != nil {
			logger.PrintErr(err)
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		// contextにトークンを格納
		c.Set("claim", claim)
		// トークンを格納
		c.Set("token", token)
		// ユーザーIDを格納
		c.Set("UserID", claim.UserID)

		// 認証処理
		return next(c)
	}
}
