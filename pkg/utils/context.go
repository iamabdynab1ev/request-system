package utils

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
)

func ContextWithTimeout(ctx echo.Context, timeout int) (context.Context, context.CancelFunc) {
	reqCtx := ctx.Request().Context()
	ctxWithTimeOut, cancelContext := context.WithTimeout(reqCtx, time.Duration(timeout)*time.Second)

	return ctxWithTimeOut, cancelContext
}