package services

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/zap"

	"request-system/internal/entities"
	"request-system/internal/repositories"
)

func TestGetTimelineByOrderID_HumanizesStructureChangeComment(t *testing.T) {
	comment := "Смена структуры: department_id: 1 -> 2; otdel_id: -> 19"
	service := &OrderHistoryService{
		repo: &orderHistoryRepoStub{
			events: []repositories.OrderHistoryItem{
				{OrderID: 1, UserID: 1, EventType: "STRUCTURE_CHANGE", Comment: nullString(comment), CreatedAt: historyTime(1)},
			},
		},
		userRepo: &historyUserLookupStub{
			users: map[uint64]entities.User{
				1: {ID: 1, Fio: "Автор"},
			},
		},
		departmentRepo: &historyDepartmentLookupStub{
			names: map[uint64]string{
				1: "Старый департамент",
				2: "Новый департамент",
			},
		},
		otdelRepo: &historyOtdelLookupStub{
			names: map[uint64]string{
				19: "Сектор банкоматов",
			},
		},
		statusRepo:   &historyStatusLookupStub{},
		priorityRepo: &historyPriorityLookupStub{},
		logger:       zap.NewNop(),
	}

	timeline, err := service.GetTimelineByOrderID(context.Background(), 1, "", "")
	if err != nil {
		t.Fatalf("GetTimelineByOrderID returned error: %v", err)
	}
	if len(timeline) != 1 {
		t.Fatalf("expected one timeline block, got %d", len(timeline))
	}
	if len(timeline[0].Lines) != 1 {
		t.Fatalf("expected one timeline line, got %d", len(timeline[0].Lines))
	}

	want := "Смена структуры: департамент: «Старый департамент» → «Новый департамент»; отдел: «Сектор банкоматов»"
	if timeline[0].Lines[0] != want {
		t.Fatalf("expected structure change line %q, got %q", want, timeline[0].Lines[0])
	}
}

func TestHumanizeHistoryStructureComment_HumanizesKnownFieldsWithoutNames(t *testing.T) {
	comment := "Смена структуры: department_id: 1 -> 2; branch_id: -> 3"

	got := humanizeHistoryStructureComment(comment, nil)
	prefix := strings.SplitN(comment, ":", 2)[0] + ":"
	departmentLabel, _ := historyStructureFieldLabel("department_id")
	branchLabel, _ := historyStructureFieldLabel("branch_id")
	want := prefix + " " + fmt.Sprintf("%s: 1 → 2; %s: 3", departmentLabel, branchLabel)
	if got != want {
		t.Fatalf("expected humanized comment %q, got %q", want, got)
	}
}

func TestHumanizeHistoryStructureComment_HandlesLegacyMojibakeArrow(t *testing.T) {
	comment := "Смена структуры: otdel_id: в†’ 19"

	got := humanizeHistoryStructureComment(comment, func(field string, id uint64) string {
		if field == "otdel_id" && id == 19 {
			return "Сектор банкоматов"
		}
		return ""
	})

	want := "Смена структуры: отдел: «Сектор банкоматов»"
	if got != want {
		t.Fatalf("expected humanized legacy arrow comment %q, got %q", want, got)
	}
}
