// Файл: authz/authz.go (ФИНАЛЬНАЯ, ГИБКАЯ ВЕРСИЯ)

package authz

import (
	"strings"

	"request-system/internal/entities"
)

// --- РАЗДЕЛ 1: СТРУКТУРЫ И ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ (без изменений) ---

type Context struct {
	Actor         *entities.User
	Permissions   map[string]bool
	Target        interface{}
	IsParticipant bool 
}

func (c *Context) HasPermission(permission string) bool {
	if c.Permissions == nil {
		return false
	}
	return c.Permissions[permission]
}

func getAction(permission string) string {
	parts := strings.Split(permission, ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func CanDo(permission string, ctx Context) bool {
	if getAction(permission) == "view" && ctx.HasPermission(ScopeAllView) {
		return true
	}

	if ctx.HasPermission(ScopeAll) && ctx.HasPermission(permission) {
		return true
	}

	if !ctx.HasPermission(permission) {
		return false
	}

	if ctx.Target == nil {
		return true
	}

	switch target := ctx.Target.(type) {
	case *entities.Order:

		return canAccessOrder(ctx, target)

	case *entities.User:

		return canAccessUser(ctx, target)

	}

	return false
}

func canAccessUser(ctx Context, target *entities.User) bool {
	if ctx.HasPermission(ProfileUpdate) || ctx.HasPermission(PasswordUpdate) {
		if ctx.Actor.ID == target.ID {
			return true
		}
	}

	return false
}

func canAccessOrder(ctx Context, target *entities.Order) bool {
	actor := ctx.Actor

	isCreator := actor.ID == target.CreatorID
	isExecutor := target.ExecutorID != nil && actor.ID == *target.ExecutorID
	isParticipant := ctx.IsParticipant || isCreator || isExecutor
	if ctx.HasPermission(ScopeOwn) && isParticipant {
		return true
	}

	if ctx.HasPermission(ScopeDepartment) && actor.DepartmentID == target.DepartmentID {
		if ctx.HasPermission(OrdersView) || ctx.HasPermission(OrdersUpdateInDepartmentScope) {
			return true
		}
	}
	// Уровень Филиала
	if ctx.HasPermission(ScopeBranch) && target.BranchID != nil && actor.BranchID == *target.BranchID {
		if ctx.HasPermission(OrdersView) || ctx.HasPermission(OrdersUpdateInBranchScope) {
			return true
		}
	}
	// Уровень Офиса
	if ctx.HasPermission(ScopeOffice) && actor.OfficeID != nil && target.OfficeID != nil && *actor.OfficeID == *target.OfficeID {
		if ctx.HasPermission(OrdersView) || ctx.HasPermission(OrdersUpdateInOfficeScope) {
			return true
		}
	}
	// Уровень Отдела
	if ctx.HasPermission(ScopeOtdel) && actor.OtdelID != nil && target.OtdelID != nil && *actor.OtdelID == *target.OtdelID {
		if ctx.HasPermission(OrdersView) || ctx.HasPermission(OrdersUpdateInOtdelScope) {
			return true
		}
	}

	return false
}
