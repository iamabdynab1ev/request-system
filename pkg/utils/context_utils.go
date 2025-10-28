package utils

import (
	"context"
	"strings"

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

func HasPermission[T any](claims *T, requiredPermission string) bool {
	if claims == nil {
		return false
	}

	userPerms := make(map[string]bool)
	if userPerms["superuser"] {
		return true
	}

	if userPerms["order:manage:all"] && strings.HasPrefix(requiredPermission, "order:view:") {
		return true
	}

	if userPerms[requiredPermission] {
		return true
	}

	if userPerms["Manage:All"] {
		return true
	}
	parts := strings.SplitN(requiredPermission, ":", 2)
	if len(parts) == 2 {
		entityName := parts[1]
		if subParts := strings.Split(entityName, ":"); len(subParts) > 1 {
			entityName = subParts[0]
		}
		managePermission := "Manage:" + entityName
		if userPerms[managePermission] {
			return true
		}
		if userPerms["Manage:System"] {
			if entityName == "Role" || entityName == "Permission" || entityName == "Catalog" {
				return true
			}
		}
	}
	if userPerms["View:Order:All"] && strings.HasPrefix(requiredPermission, "View:Order:") {
		return true
	}
	if requiredPermission == "View:Order:Own" && userPerms["View:Order:Department"] {
		return true
	}

	return false
}
