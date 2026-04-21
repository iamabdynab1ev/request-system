package authz

import (
	"testing"

	"request-system/internal/entities"
)

func TestCanDoOrderViewAllowsParticipantWithOwnScope(t *testing.T) {
	actor := &entities.User{ID: 10}
	order := &entities.Order{ID: 100, CreatorID: 20}

	ctx := Context{
		Actor:         actor,
		Target:        order,
		IsParticipant: true,
		Permissions: map[string]bool{
			OrdersView: true,
			ScopeOwn:   true,
		},
	}

	if !CanDo(OrdersView, ctx) {
		t.Fatal("expected participant with own scope to view order")
	}
}

func TestCanDoOrderUpdateRejectsParticipantWithoutOwnership(t *testing.T) {
	actor := &entities.User{ID: 10}
	order := &entities.Order{ID: 100, CreatorID: 20}

	ctx := Context{
		Actor:         actor,
		Target:        order,
		IsParticipant: true,
		Permissions: map[string]bool{
			OrdersUpdate: true,
		},
	}

	if CanDo(OrdersUpdate, ctx) {
		t.Fatal("expected participant without creator/executor ownership not to update order")
	}
}

func TestCanDoOrderUpdateAllowsCurrentExecutor(t *testing.T) {
	actor := &entities.User{ID: 10}
	executorID := actor.ID
	order := &entities.Order{ID: 100, CreatorID: 20, ExecutorID: &executorID}

	ctx := Context{
		Actor:  actor,
		Target: order,
		Permissions: map[string]bool{
			OrdersUpdate: true,
		},
	}

	if !CanDo(OrdersUpdate, ctx) {
		t.Fatal("expected current executor to update order")
	}
}

func TestCanDoOrderUpdateAllowsDepartmentScopeMatch(t *testing.T) {
	departmentID := uint64(5)
	actor := &entities.User{ID: 10, DepartmentID: &departmentID}
	order := &entities.Order{ID: 100, CreatorID: 20, DepartmentID: &departmentID}

	ctx := Context{
		Actor:  actor,
		Target: order,
		Permissions: map[string]bool{
			OrdersUpdate:                  true,
			OrdersUpdateInDepartmentScope: true,
		},
	}

	if !CanDo(OrdersUpdate, ctx) {
		t.Fatal("expected department-scoped user to update order in same department")
	}
}
