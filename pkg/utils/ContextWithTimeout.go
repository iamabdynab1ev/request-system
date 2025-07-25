package utils

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
)


func Ctx(c echo.Context, seconds int) context.Context {
	newCtx, cancel := context.WithTimeout(c.Request().Context(), time.Duration(seconds)*time.Second)

	// Автоматический cancel после завершения
	go func() {
		<-newCtx.Done()
		cancel()
	}()

	return newCtx
}
