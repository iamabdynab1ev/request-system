// Файл: pkg/utils/context_utils.go

package utils

import (
	"context"
	"request-system/internal/dto"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"strings"
)

func GetClaimsFromContext(ctx context.Context) (*dto.UserClaims, error) {
	claims, ok := ctx.Value(contextkeys.UserIDKey).(*dto.UserClaims)
	if !ok || claims == nil {
		return nil, apperrors.ErrUnauthorized
	}
	return claims, nil
}

func HasPermission(claims *dto.UserClaims, requiredPermission string) bool {
	if claims == nil {
		return false
	}

	userPerms := make(map[string]bool)
	for _, p := range claims.Permissions {
		userPerms[p] = true
	}

	if userPerms["Manage:All"] {
		return true
	}

	if userPerms[requiredPermission] {
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
			if entityName == "Roles" || entityName == "Permissions" || entityName == "Catalogs" {
				return true
			}
		}
	}

	if userPerms["View:Orders:All"] && strings.HasPrefix(requiredPermission, "View:Orders:") {
		return true
	}
	if requiredPermission == "View:Orders:Own" && userPerms["View:Orders:Department"] {
		return true
	}

	return false
}
