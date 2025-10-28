// Файл: internal/authz/authz.go
package authz

import (
	"strings"

	"request-system/internal/entities"
)

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

func canAccessOrder(ctx Context, target *entities.Order) bool {
	if ctx.HasPermission(ScopeAll) || ctx.HasPermission(ScopeAllView) {
		return true
	}
	actor := ctx.Actor

	if ctx.HasPermission(ScopeDepartment) && actor.DepartmentID > 0 && target.DepartmentID == actor.DepartmentID {
		return true
	}
	if ctx.HasPermission(ScopeBranch) && actor.BranchID != nil && target.BranchID != nil && *actor.BranchID == *target.BranchID {
		return true
	}
	if ctx.HasPermission(ScopeOffice) && actor.OfficeID != nil && target.OfficeID != nil && *actor.OfficeID == *target.OfficeID {
		return true
	}
	if ctx.HasPermission(ScopeOtdel) && actor.OtdelID != nil && target.OtdelID != nil && *actor.OtdelID == *target.OtdelID {
		return true
	}
	if ctx.IsParticipant {
		return true
	}
	return false
}

func canAccessUser(ctx Context, target *entities.User) bool {
	actor := ctx.Actor

	// Правило 1: Пользователь всегда может редактировать/просматривать себя, если у него есть права на профиль.
	if actor.ID == target.ID {
		if ctx.HasPermission(ProfileUpdate) || ctx.HasPermission(PasswordUpdate) || ctx.HasPermission(UsersView) {
			return true
		}
	}

	// Правило 2: Пользователь с `scope:all` может делать что угодно с другими.
	if ctx.HasPermission(ScopeAll) {
		return true
	}

	// Правило 3: Проверка иерархических scope (если они нужны для пользователей)
	// Эти проверки имеют смысл, если вы хотите, чтобы руководитель отдела мог редактировать своих сотрудников.
	if ctx.HasPermission(ScopeDepartment) && actor.DepartmentID == target.DepartmentID {
		return true
	}
	if ctx.HasPermission(ScopeBranch) && actor.BranchID != nil && target.BranchID != nil && *actor.BranchID == *target.BranchID {
		return true
	}
	// ... (добавьте ScopeOtdel, ScopeOffice по аналогии, если нужно) ...

	// Если ни одно из правил не сработало
	return false
}

func CanDo(permission string, ctx Context) bool {
	if !ctx.HasPermission(permission) {
		return false
	}
	if ctx.Target == nil {
		return true
	}

	switch target := ctx.Target.(type) {
	case *entities.Order:

		if !canAccessOrder(ctx, target) {
			return false
		}

		switch permission {
		case OrdersUpdateInDepartmentScope:
			return ctx.HasPermission(ScopeDepartment)
		case OrdersUpdateInBranchScope:
			return ctx.HasPermission(ScopeBranch)
		case OrdersUpdateInOfficeScope:
			return ctx.HasPermission(ScopeOffice)
		case OrdersUpdateInOtdelScope:
			return ctx.HasPermission(ScopeOtdel)
		}

		return true

	case *entities.User:

		return canAccessUser(ctx, target)
	}

	return false
}
