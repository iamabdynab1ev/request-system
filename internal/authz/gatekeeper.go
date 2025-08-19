package authz

import (
	"request-system/internal/entities"
	"strings"
)

// Gatekeeper остается пустым, это просто "контейнер" для методов
type Gatekeeper struct{}

func NewGatekeeper() *Gatekeeper {
	return &Gatekeeper{}
}

// Can - теперь принимает `perms` ОТДЕЛЬНО от `actor`.
func (g *Gatekeeper) Can(
	perms map[string]bool,
	actor *entities.User,
	permission string,
	target interface{},
) bool {
	// Этап 1: Проверка на Superuser
	if perms[Superuser] {
		return true
	}

	// Этап 2: Проверка на наличие базового пермишена
	if !perms[permission] {
		return false
	}

	// Этап 3: Проверка для простых сущностей
	isSimpleEntity := strings.HasPrefix(permission, "catalogs:") ||
		strings.HasPrefix(permission, "structure:") ||
		strings.HasPrefix(permission, "roles:") ||
		strings.HasPrefix(permission, "permissions:")

	if isSimpleEntity {
		if strings.HasSuffix(permission, ":view") {
			return true
		}
		return perms[ScopeAll]
	}

	// Этап 4: Глубокая проверка для сложных сущностей

	if target == nil {
		return perms[ScopeAll] || perms[ScopeDepartment] || perms[ScopeOwn]
	}

	if perms[ScopeAll] {
		return true
	}

	switch t := target.(type) {
	case *entities.Order:
		if perms[ScopeDepartment] && actor.DepartmentID == t.DepartmentID {
			return true
		}
		if perms[ScopeOwn] && (actor.ID == t.CreatorID || actor.ID == t.ExecutorID) {
			return true
		}

	case *entities.User:
		if perms[ScopeDepartment] && actor.DepartmentID == t.DepartmentID {
			return true
		}
		if perms[ScopeOwn] && actor.ID == t.ID {
			return true
		}
	}

	return false
}
