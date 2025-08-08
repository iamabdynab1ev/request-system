package authz

import (
	"request-system/internal/entities"
	"strings"
)


type Context struct {
	Actor       *entities.User  
	Permissions map[string]bool 
	Target      interface{}     
}

func CanDo(permission string, ctx Context) bool {
	if ctx.Permissions[Superuser] {
		return true
	}

	if !ctx.Permissions[permission] {
		return false
	}

	isMgmtPermission := strings.HasPrefix(permission, "roles:") ||
		strings.HasPrefix(permission, "permissions:") ||
		strings.HasPrefix(permission, "structure:") ||
		strings.HasPrefix(permission, "catalogs:")

	if isMgmtPermission {
		if strings.HasSuffix(permission, ":view") {
			return true
		}
		return ctx.Permissions[ScopeAll]
	}

	if ctx.Permissions[ScopeAll] {
		return true
	}

	if ctx.Target == nil {
		return ctx.Permissions[ScopeDepartment] || ctx.Permissions[ScopeOwn]
	}

	switch target := ctx.Target.(type) {
	case *entities.Order:
		if ctx.Permissions[ScopeDepartment] && ctx.Actor.DepartmentID == target.DepartmentID {
			return true
		}
		if ctx.Permissions[ScopeOwn] && (ctx.Actor.ID == target.CreatorID || ctx.Actor.ID == target.ExecutorID) {
			return true
		}

	case *entities.User:
		if ctx.Permissions[ScopeDepartment] && ctx.Actor.DepartmentID == target.DepartmentID {
			return true
		}
		if ctx.Permissions[ScopeOwn] && ctx.Actor.ID == target.ID {
			return true
		}
	}

	return false
}
