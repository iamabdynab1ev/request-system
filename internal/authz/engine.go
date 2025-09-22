package authz

import (
	"slices"
	"strings"

	"request-system/internal/entities"
)

// --- РАЗДЕЛ 1: СТРУКТУРЫ И ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ---
// Этот раздел отвечает за подготовку данных для проверки прав.

// Context содержит всю информацию, необходимую для принятия решения о доступе.
type Context struct {
	Actor       *entities.User
	Permissions map[string]bool
	Target      interface{}
}

// HasPermission - это простой хелпер для безопасной проверки наличия права в карте.
func (c *Context) HasPermission(permission string) bool {
	if c.Permissions == nil {
		return false
	}
	return c.Permissions[permission]
}

// simpleEntities - это список "простых" сущностей (справочников).
// Для них действуют упрощенные правила доступа.
var simpleEntities = []string{
	"role", "permission", "status", "priority", "department", "otdel",
	"branch", "office", "equipment", "equipment_type", "position",
}

// isSimpleEntity проверяет, относится ли право к простому справочнику.
func isSimpleEntity(permission string) bool {
	entity := strings.Split(permission, ":")[0]
	return slices.Contains(simpleEntities, entity)
}

// getAction извлекает действие из права (например, 'update' из 'order:update').
func getAction(permission string) string {
	parts := strings.Split(permission, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// --- РАЗДЕЛ 2: ГЛАВНАЯ ЛОГИКА АВТОРИЗАЦИИ ---
// Эта функция - "мозг" всей системы прав. Она вызывается из сервисов.

func CanDo(permission string, ctx Context) bool {
	// ПРАВИЛО №1: Superuser может абсолютно всё.
	// Это самая первая и главная проверка.
	if ctx.HasPermission(Superuser) {
		return true
	}

	// ПРАВИЛО №2 (СПЕЦИАЛЬНОЕ): Упрощенная логика для просмотра пользователей.
	// Если мы проверяем право "user:view", то вся сложная логика со scope'ами ниже
	// игнорируется. Достаточно просто иметь это одно право.
	if permission == UsersView {
		return ctx.HasPermission(UsersView)
	}

	// ПРАВИЛО №3: Проверка наличия базового права.
	// Если у пользователя нет самого простого права на действие, дальнейшие проверки бессмысленны.
	if !ctx.HasPermission(permission) {
		return false
	}

	action := getAction(permission)

	// ПРАВИЛО №3.1: Для действия 'create' достаточно базового права.
	if action == "create" {
		return true
	}

	// ПРАВИЛО №4: Логика для простых справочников.
	if isSimpleEntity(permission) {
		// Смотреть ('view') справочники можно, если есть базовое право.
		if action == "view" {
			return true
		}
		// А изменять/удалять справочники может только тот, у кого есть глобальный доступ (`scope:all`).
		return ctx.HasPermission(ScopeAll)
	}

	// ПРАВИЛО №5: Сложная логика на основе SCOPE для основных сущностей (Заявки).
	// Если у пользователя есть `scope:all`, он может выполнять действие над любым объектом.
	if ctx.HasPermission(ScopeAll) {
		return true
	}

	// Если `Target` не указан, значит, это запрос на СПИСОК (например, GET /api/orders).
	// Чтобы видеть хоть что-то, пользователь должен иметь хотя бы один scope.
	if ctx.Target == nil {
		return ctx.HasPermission(ScopeDepartment) || ctx.HasPermission(ScopeOwn)
	}

	// Если `Target` указан, это запрос на КОНКРЕТНЫЙ объект.
	switch target := ctx.Target.(type) {
	case *entities.Order: // ПРАВИЛА ДЛЯ ЗАЯВОК
		// Разрешено, если у пользователя есть scope отдела и заявка из его отдела.
		if ctx.HasPermission(ScopeDepartment) && ctx.Actor.DepartmentID == target.DepartmentID {
			return true
		}
		// Разрешено, если у пользователя есть scope "своих данных" и он является создателем ИЛИ исполнителем.
		if ctx.HasPermission(ScopeOwn) {
			isCreator := ctx.Actor.ID == target.CreatorID
			isExecutor := target.ExecutorID != nil && ctx.Actor.ID == *target.ExecutorID
			if isCreator || isExecutor {
				return true
			}
		}

	case *entities.User:
		// Эти правила теперь будут работать для `user:update`, `user:delete` и т.д.
		if ctx.HasPermission(ScopeDepartment) && ctx.Actor.DepartmentID == target.DepartmentID {
			return true
		}
		if ctx.HasPermission(ScopeOwn) && ctx.Actor.ID == target.ID {
			return true
		}
	}

	// ПРАВИЛО №6 (ПО УМОЛЧАНИЮ): Если ни одно из правил выше не разрешило доступ, запретить.
	return false
}
