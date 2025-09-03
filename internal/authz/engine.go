package authz

import (
	"strings"

	"request-system/internal/entities"
)

// Context содержит все данные, необходимые для принятия решения о доступе.
type Context struct {
	Actor       *entities.User
	Permissions map[string]bool
	Target      interface{}
}

// --- >>> ДОБАВЛЕНО <<< ---
// HasPermission - это удобный хелпер для проверки, есть ли у пользователя конкретное право.
func (c *Context) HasPermission(permission string) bool {
	if c.Permissions == nil {
		return false
	}
	return c.Permissions[permission]
}

// --- >>> КОНЕЦ ДОБАВЛЕНИЯ <<< ---

// CanDo - наша единая "умная" функция проверки прав.
func CanDo(permission string, ctx Context) bool {
	// Этап 1: Superuser может абсолютно всё.
	if ctx.HasPermission(Superuser) { // Используем новый хелпер для чистоты кода
		return true
	}

	// Специальное правило: Разрешаем просмотр профиля любого пользователя
	if permission == UsersView && ctx.Target != nil {
		if _, ok := ctx.Target.(*entities.User); ok {
			return ctx.HasPermission(UsersView) // Проверяем, что базовое право на просмотр все-таки есть
		}
	}

	// Этап 2: Проверяем, есть ли у пользователя базовый пермишен на само действие.
	if !ctx.HasPermission(permission) { // Используем новый хелпер
		return false
	}

	action := getActionFromPermission(permission)

	// Правило 2.1: Для 'create' достаточно базового пермишена.
	if action == "create" {
		return true
	}

	// Этап 3: Проверяем простые сущности (справочники).
	isSimpleEntityPermission := strings.HasPrefix(permission, "role:") ||
		strings.HasPrefix(permission, "permission:") ||
		strings.HasPrefix(permission, "status:") ||
		strings.HasPrefix(permission, "priority:") ||
		strings.HasPrefix(permission, "department:") ||
		strings.HasPrefix(permission, "otdel:") ||
		strings.HasPrefix(permission, "branch:") ||
		strings.HasPrefix(permission, "office:") ||
		strings.HasPrefix(permission, "equipment:") ||
		strings.HasPrefix(permission, "equipment_type:") ||
		strings.HasPrefix(permission, "position:")

	if isSimpleEntityPermission {
		if action == "view" {
			return true
		}
		return ctx.HasPermission(ScopeAll) // Для управления справочниками нужен scope:all
	}

	// Этап 4: Применяем сложную логику scope для Заявок и Пользователей.
	if ctx.HasPermission(ScopeAll) {
		return true
	}

	if ctx.Target == nil { // Запрос на список
		return ctx.HasPermission(ScopeDepartment) || ctx.HasPermission(ScopeOwn)
	}

	// Запрос на конкретный объект
	switch target := ctx.Target.(type) {
	case *entities.Order: // Работа с Заявкой
		if ctx.HasPermission(ScopeDepartment) && ctx.Actor.DepartmentID == target.DepartmentID {
			return true
		}
		if ctx.HasPermission(ScopeOwn) && (ctx.Actor.ID == target.CreatorID || ctx.Actor.ID == target.ExecutorID) {
			return true
		}

	case *entities.User: // Работа с Пользователем
		if ctx.HasPermission(ScopeDepartment) && ctx.Actor.DepartmentID == target.DepartmentID {
			return true
		}
		if ctx.HasPermission(ScopeOwn) && ctx.Actor.ID == target.ID {
			return true
		}
	}

	return false // Если ни одно правило не сработало
}

// getActionFromPermission - приватный хелпер для извлечения действия (без изменений).
func getActionFromPermission(permission string) string {
	parts := strings.Split(permission, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}
