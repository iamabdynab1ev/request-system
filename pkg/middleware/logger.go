// pkg/middleware/logger.go

package middleware

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// InjectLogger - мидлвэр для добавления логгера в контекст запроса.
func InjectLogger(logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			
			c.Set("logger", logger)
			return next(c)
		}
	}
}
