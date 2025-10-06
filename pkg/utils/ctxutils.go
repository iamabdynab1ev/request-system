// pkg/utils/auth_helpers.go

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

func GetUserRoleIDFromCtx(ctx context.Context) (uint64, error) {
	roleID, ok := ctx.Value(contextkeys.RoleIDKey).(uint64)
	if !ok {
		return 0, apperrors.ErrUserNotFound
	}
	return roleID, nil
}

func GetPermissionsMapFromCtx(ctx context.Context) (map[string]bool, error) {
	permissions, ok := ctx.Value(contextkeys.UserPermissionsMapKey).(map[string]bool)
	if !ok || permissions == nil {
		return nil, apperrors.ErrForbidden
	}
	return permissions, nil
}
