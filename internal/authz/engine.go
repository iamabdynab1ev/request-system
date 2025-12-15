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
	parts := strings.Split(permission, ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// canAccessOrder — логика для Заявок (СТРОГАЯ)
func canAccessOrder(ctx Context, target *entities.Order) bool {
	action := getAction(ctx.CurrentPermission)
	actor := ctx.Actor

	// Просмотр заявки
	if action == "view" {
		// Глобальные права видят всё
		if ctx.HasPermission(ScopeAllView) || ctx.HasPermission(ScopeAll) {
			return true
		}
		// Проверка по иерархии (совпадает ли департамент/отдел/филиал)
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
		// Собственные заявки (участие)
		if ctx.HasPermission(ScopeOwn) {
			isCreator := target.CreatorID == actor.ID
			isExecutor := target.ExecutorID != nil && *target.ExecutorID == actor.ID
			if isCreator || isExecutor || ctx.IsParticipant {
				return true
			}
		}
		return false
	}

	// Редактирование (update/delete)

	// Админ может всё
	if ctx.HasPermission(ScopeAll) {
		return true
	}

	// Управленческий доступ (например, начальник департамента может править заявки в своем департаменте)
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

	// Свои заявки (обычный пользователь может править, если он Создатель или Исполнитель)
	if ctx.HasPermission(OrdersUpdate) {
		isCreator := target.CreatorID == actor.ID
		isExecutor := target.ExecutorID != nil && *target.ExecutorID == actor.ID
		if isCreator || isExecutor || ctx.IsParticipant {
			return true
		}
	}

	return false
}

// canAccessUser — логика для Пользователей ("ТЕЛЕФОННАЯ КНИГА" + СТРОГОСТЬ)
func canAccessUser(ctx Context, target *entities.User) bool {
	actor := ctx.Actor
	action := getAction(ctx.CurrentPermission)

	// Правило 1: Сам себя вижу и правлю (если есть базовые права)
	if actor.ID == target.ID {
		return true
	}

	// Правило 2: Админ
	if ctx.HasPermission(ScopeAll) {
		return true
	}

	// Правило 3 (НОВОЕ): Глобальный просмотр.
	// Если действие == view, мы разрешаем доступ к карточке любого сотрудника.
	// Это нужно, чтобы выбирать пользователей из списка, даже если они в другом отделе.
	if action == "view" {
		return true
	}

	// === ДЛЯ РЕДАКТИРОВАНИЯ/УДАЛЕНИЯ — СТРОГАЯ ИЕРАРХИЯ ===

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
	// 1. Фиксация права
	ctx.CurrentPermission = permission

	// 2. Есть ли право вообще (RBAC)
	if !ctx.HasPermission(permission) {
		return false
	}

	// 3. Без цели — разрешено (например создание)
	if ctx.Target == nil {
		return true
	}

	// 4. Проверка цели (ABAC)
	switch target := ctx.Target.(type) {
	case *entities.Order:
		return canAccessOrder(ctx, target)
	case *entities.User:
		return canAccessUser(ctx, target)
	}

	return true
}
