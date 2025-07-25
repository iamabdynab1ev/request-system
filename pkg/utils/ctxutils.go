package utils

import (
	"context"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
)

func GetUserIDFromCtx(ctx context.Context) (uint64, error) {
	userID, ok := ctx.Value(contextkeys.UserIDKey).(uint64)
	if !ok {
		return 0, apperrors.ErrUserNotFound
	}
	return userID, nil
}
