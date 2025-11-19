package utils

import (
	"context"

	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
)

func GetClaimsFromContext[T any](ctx context.Context) (*T, error) {
	claims, ok := ctx.Value(contextkeys.UserIDKey).(*T)
	if !ok || claims == nil {
		return nil, apperrors.ErrUnauthorized
	}
	return claims, nil
}
