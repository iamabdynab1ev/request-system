package repositories

import (
	"reflect"
	"strings"
	"testing"

	pkgconstants "request-system/pkg/constants"
)

func TestDashboardStatusSets(t *testing.T) {
	wantClosed := []string{pkgconstants.StatusClosed}
	if !reflect.DeepEqual(dashboardClosedStatuses, wantClosed) {
		t.Fatalf("unexpected closed statuses: got %v want %v", dashboardClosedStatuses, wantClosed)
	}

	wantResolved := []string{pkgconstants.StatusClosed}
	if !reflect.DeepEqual(dashboardResolvedStatuses, wantResolved) {
		t.Fatalf("unexpected resolved statuses: got %v want %v", dashboardResolvedStatuses, wantResolved)
	}

	wantOpenExcluded := []string{pkgconstants.StatusClosed}
	if !reflect.DeepEqual(dashboardOpenExcludedStatuses, wantOpenExcluded) {
		t.Fatalf("unexpected open excluded statuses: got %v want %v", dashboardOpenExcludedStatuses, wantOpenExcluded)
	}
}

func TestDashboardStatusSQLChecks(t *testing.T) {
	if got := dashboardStatusInCheck("status_code", dashboardClosedStatuses); got != "status_code IN ('CLOSED')" {
		t.Fatalf("unexpected closed check: %s", got)
	}

	if got := dashboardStatusInCheck("status_code", dashboardResolvedStatuses); got != "status_code IN ('CLOSED')" {
		t.Fatalf("unexpected resolved check: %s", got)
	}

	if got := dashboardStatusNotInCheck("status_code", dashboardOpenExcludedStatuses); got != "status_code NOT IN ('CLOSED')" {
		t.Fatalf("unexpected open check: %s", got)
	}
}

func TestDashboardLatestStatusChangeTimestampExpr(t *testing.T) {
	got := dashboardLatestStatusChangeTimestampExpr("o", pkgconstants.StatusClosed, "closed_at")
	if !strings.Contains(got, "target_status.code = 'CLOSED'") {
		t.Fatalf("expected CLOSED status filter, got %s", got)
	}
	if !strings.Contains(got, "h.order_id = o.id") {
		t.Fatalf("expected order alias usage, got %s", got)
	}
	if !strings.Contains(got, "AS closed_at") {
		t.Fatalf("expected alias, got %s", got)
	}
}

func TestDashboardLatestStatusChangeTimestampScalarExpr(t *testing.T) {
	got := dashboardLatestStatusChangeTimestampScalarExpr("o", pkgconstants.StatusClosed)
	if strings.Contains(got, "AS closed_at") {
		t.Fatalf("did not expect alias in scalar expr: %s", got)
	}
	if !strings.Contains(got, "target_status.code = 'CLOSED'") {
		t.Fatalf("expected CLOSED status filter, got %s", got)
	}
}

func TestDashboardSLAHelpers(t *testing.T) {
	if got := dashboardSLAEligibleCheck("duration"); got != "duration IS NOT NULL" {
		t.Fatalf("unexpected SLA eligible check: %s", got)
	}

	if got := dashboardSLAOnTimeCheck("duration", "completed_at"); got != "duration IS NOT NULL AND completed_at <= duration" {
		t.Fatalf("unexpected SLA on-time check: %s", got)
	}
}

func TestDashboardActiveAgentHelpers(t *testing.T) {
	if got := dashboardEventNotInCheck("h.event_type", dashboardActiveAgentExcludedEvents); got != "h.event_type NOT IN ('CREATE', 'DELEGATION')" {
		t.Fatalf("unexpected active-agent event check: %s", got)
	}

	got := dashboardLatestDelegationAssigneeCheck("h")
	if !strings.Contains(got, "assign.event_type = 'DELEGATION'") {
		t.Fatalf("expected delegation filter, got %s", got)
	}
	if !strings.Contains(got, "ORDER BY assign.created_at DESC") {
		t.Fatalf("expected latest delegation lookup, got %s", got)
	}
	if !strings.Contains(got, "= h.user_id::text") {
		t.Fatalf("expected delegated assignee comparison, got %s", got)
	}
}

func TestHumanizeDashboardStructureComment_OtdelAssignment(t *testing.T) {
	comment := "Смена структуры: otdel_id: → 19"
	got := humanizeDashboardStructureComment(comment, func(field string, id uint64) string {
		if field == "otdel_id" && id == 19 {
			return "Сектор банкоматов"
		}
		return ""
	})

	want := "Смена структуры: отдел: «Сектор банкоматов»"
	if got != want {
		t.Fatalf("unexpected humanized structure change: got %q want %q", got, want)
	}
}

func TestHumanizeDashboardStructureComment_MultipleFields(t *testing.T) {
	comment := "Смена структуры: department_id: 1 → 2; office_id:  → 5"
	got := humanizeDashboardStructureComment(comment, func(field string, id uint64) string {
		switch {
		case field == "department_id" && id == 1:
			return "Старый департамент"
		case field == "department_id" && id == 2:
			return "Новый департамент"
		case field == "office_id" && id == 5:
			return "Головной офис"
		default:
			return ""
		}
	})

	want := "Смена структуры: департамент: «Старый департамент» → «Новый департамент»; офис: «Головной офис»"
	if got != want {
		t.Fatalf("unexpected multi-field structure change: got %q want %q", got, want)
	}
}
