// Файл: internal/authz/authz.go
package authz

import (
	"strings"

	"request-system/internal/entities"
)

type Context struct {
	Actor             *entities.User
	Permissions       map[string]bool
	Target            interface{}
	IsParticipant     bool
	CurrentPermission string
}

func (c *Context) HasPermission(permission string) bool {
	if c.Permissions == nil {
		return false
	}
	_, exists := c.Permissions[permission]
	return exists
}

func getAction(permission string) string {
	if parts := strings.Split(permission, ":"); len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func canAccessOrder(ctx Context, target *entities.Order) bool {
	action := getAction(ctx.CurrentPermission)
	actor := ctx.Actor
	if action == "view" {
		if ctx.HasPermission(ScopeAllView) || ctx.HasPermission(ScopeAll) {
			return true
		}
		if ctx.HasPermission(ScopeDepartment) && actor.DepartmentID != nil && target.DepartmentID != nil && *actor.DepartmentID == *target.DepartmentID {
			return true
		}
		if ctx.HasPermission(ScopeBranch) && actor.BranchID != nil && target.BranchID != nil && *actor.BranchID == *target.BranchID {
			return true
		}
		if ctx.HasPermission(ScopeOtdel) && actor.OtdelID != nil && target.OtdelID != nil && *actor.OtdelID == *target.OtdelID {
			return true
		}
		if ctx.HasPermission(ScopeOffice) && actor.OfficeID != nil && target.OfficeID != nil && *actor.OfficeID == *target.OfficeID {
			return true
		}
		if ctx.HasPermission(ScopeOwn) {
			isCreator := target.CreatorID == actor.ID
			isExecutor := target.ExecutorID != nil && *target.ExecutorID == actor.ID
			if isCreator || isExecutor || ctx.IsParticipant {
				return true
			}
		}
		return false
	}

	// Уровень 1: Полный доступ на редактирование
	if ctx.HasPermission(ScopeAll) {
		return true
	}

	// Уровень 2: Управленческий доступ
	if ctx.HasPermission(OrdersUpdateInDepartmentScope) && actor.DepartmentID != nil && target.DepartmentID != nil && *actor.DepartmentID == *target.DepartmentID {
		return true
	}
	if ctx.HasPermission(OrdersUpdateInBranchScope) && actor.BranchID != nil && target.BranchID != nil && *actor.BranchID == *target.BranchID {
		return true
	}
	if ctx.HasPermission(OrdersUpdateInOtdelScope) && actor.OtdelID != nil && target.OtdelID != nil && *actor.OtdelID == *target.OtdelID {
		return true
	}
	if ctx.HasPermission(OrdersUpdateInOfficeScope) && actor.OfficeID != nil && target.OfficeID != nil && *actor.OfficeID == *target.OfficeID {
		return true
	}

	// Уровень 3: Собственный доступ
	if ctx.HasPermission(OrdersUpdate) {
		isCreator := target.CreatorID == actor.ID
		isExecutor := target.ExecutorID != nil && *target.ExecutorID == actor.ID
		if isCreator || isExecutor || ctx.IsParticipant {
			return true
		}
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

	if ctx.HasPermission(ScopeDepartment) && actor.DepartmentID != nil && target.DepartmentID != nil && *actor.DepartmentID == *target.DepartmentID {
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

	return false
}

func CanDo(permission string, ctx Context) bool {
	// 1. Запоминаем, какое право мы проверяем. Это нужно для `canAccessOrder`.
	ctx.CurrentPermission = permission

	// 2. Базовая проверка: есть ли у пользователя вообще такое право в списке
	if !ctx.HasPermission(permission) {
		return false
	}

	// 3. Если нет объекта (`Target`) для проверки - доступ разрешен (например, для `Create`)
	if ctx.Target == nil {
		return true
	}

	// 4. Выбираем правильную функцию проверки в зависимости от типа объекта
	switch target := ctx.Target.(type) {
	case *entities.Order:
		return canAccessOrder(ctx, target)
	case *entities.User:
		return canAccessUser(ctx, target)
	}

	return true
}
