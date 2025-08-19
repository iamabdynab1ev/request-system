package authz

import (
	"request-system/internal/entities"
	"strings"
)

// Context содержит все данные, необходимые для принятия решения о доступе.
// Эта структура остается без изменений.
type Context struct {
	Actor       *entities.User
	Permissions map[string]bool
	Target      interface{}
}

// CanDo - наша единая "умная" функция проверки прав. ФИНАЛЬНАЯ ВЕРСИЯ.
func CanDo(permission string, ctx Context) bool {
	// Этап 1: Superuser может абсолютно всё.
	if ctx.Permissions[Superuser] {
		return true
	}

	// Этап 2: Проверяем, есть ли у пользователя базовый пермишен на само действие.
	if !ctx.Permissions[permission] {
		return false
	}

	// Определяем действие из пермишена (например, 'view', 'create')
	action := getActionFromPermission(permission)

	// Правило 2.1: Для действия 'create' достаточно базового пермишена.
	// Глубокая проверка не нужна, т.к. объекта еще не существует.
	if action == "create" {
		return true
	}

	// Этап 3: Проверяем, относится ли пермишен к "простым" сущностям
	// (роли, пермишены, и все твои справочники).
	isSimpleEntityPermission := strings.HasPrefix(permission, "roles:") ||
		strings.HasPrefix(permission, "permissions:") ||
		strings.HasPrefix(permission, "statuses:") ||
		strings.HasPrefix(permission, "priorities:") ||
		strings.HasPrefix(permission, "departments:") ||
		strings.HasPrefix(permission, "otdels:") ||
		strings.HasPrefix(permission, "branches:") ||
		strings.HasPrefix(permission, "offices:") ||
		strings.HasPrefix(permission, "equipments:") ||
		strings.HasPrefix(permission, "equipment_types:") ||
		strings.HasPrefix(permission, "positions:")

	if isSimpleEntityPermission {
		// Для просмотра (:view) таких сущностей достаточно базового права,
		// которое мы уже проверили на Этапе 2.
		if action == "view" {
			return true
		}
		// Для управления (update, delete) нужен глобальный доступ `scope:all`.
		return ctx.Permissions[ScopeAll]
	}

	// Этап 4: Если это не справочник (т.е. Заявки, Пользователи),
	// применяем сложную логику с областью видимости (scope).

	// Если у пользователя есть глобальный scope, он может всё.
	if ctx.Permissions[ScopeAll] {
		return true
	}

	// Если это запрос на список (цель не указана), то достаточно иметь
	// хоть какой-то ограниченный scope, чтобы что-то увидеть.
	if ctx.Target == nil {
		return ctx.Permissions[ScopeDepartment] || ctx.Permissions[ScopeOwn]
	}

	// Если цель (target) УКАЗАНА, проверяем, попадает ли она в область видимости.
	switch target := ctx.Target.(type) {

	case *entities.Order: // Если работаем с Заявкой
		if ctx.Permissions[ScopeDepartment] && ctx.Actor.DepartmentID == target.DepartmentID {
			return true
		}
		if ctx.Permissions[ScopeOwn] && (ctx.Actor.ID == target.CreatorID || ctx.Actor.ID == target.ExecutorID) {
			return true
		}

	case *entities.User: // Если работаем с Пользователем
		if ctx.Permissions[ScopeDepartment] && ctx.Actor.DepartmentID == target.DepartmentID {
			return true
		}
		if ctx.Permissions[ScopeOwn] && ctx.Actor.ID == target.ID {
			return true
		}
	}

	// Если ни одно из правил не сработало
	return false
}

// getActionFromPermission - приватный хелпер для извлечения действия.
func getActionFromPermission(permission string) string {
	parts := strings.Split(permission, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}
