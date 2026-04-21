package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/types"
)

func TestGetTimelineByOrderID_BatchesActorsAndCachesReferences(t *testing.T) {
	repo := &orderHistoryRepoStub{
		events: []repositories.OrderHistoryItem{
			{OrderID: 1, UserID: 1, EventType: "CREATE", NewValue: nullString("Заявка"), CreatedAt: historyTime(1)},
			{OrderID: 1, UserID: 1, EventType: "STATUS_CHANGE", NewValue: nullString("2"), NewStatusName: nullString("В работе"), CreatedAt: historyTime(2)},
			{OrderID: 1, UserID: 2, EventType: "DEPARTMENT_CHANGE", NewValue: nullString("7"), CreatedAt: historyTime(3)},
			{OrderID: 1, UserID: 2, EventType: "DEPARTMENT_CHANGE", NewValue: nullString("7"), CreatedAt: historyTime(4)},
			{OrderID: 1, UserID: 2, EventType: "PRIORITY_CHANGE", NewValue: nullString("5"), CreatedAt: historyTime(5)},
			{OrderID: 1, UserID: 2, EventType: "PRIORITY_CHANGE", NewValue: nullString("5"), CreatedAt: historyTime(6)},
		},
	}
	users := &historyUserLookupStub{
		users: map[uint64]entities.User{
			1: {ID: 1, Fio: "Создатель"},
			2: {ID: 2, Fio: "Исполнитель"},
		},
	}
	departments := &historyDepartmentLookupStub{
		names: map[uint64]string{7: "ИТ"},
	}
	priorities := &historyPriorityLookupStub{
		names: map[uint64]string{5: "Высокий"},
	}
	statuses := &historyStatusLookupStub{}

	service := &OrderHistoryService{
		repo:           repo,
		userRepo:       users,
		departmentRepo: departments,
		otdelRepo:      &historyOtdelLookupStub{},
		statusRepo:     statuses,
		priorityRepo:   priorities,
		logger:         zap.NewNop(),
	}

	timeline, err := service.GetTimelineByOrderID(context.Background(), 1, "", "")
	if err != nil {
		t.Fatalf("GetTimelineByOrderID returned error: %v", err)
	}
	if len(timeline) != len(repo.events) {
		t.Fatalf("expected %d timeline blocks, got %d", len(repo.events), len(timeline))
	}
	if users.batchCalls != 1 {
		t.Fatalf("expected one batch user lookup, got %d", users.batchCalls)
	}
	if users.byIDCalls != 0 {
		t.Fatalf("expected no per-user lookups, got %d", users.byIDCalls)
	}
	if departments.calls[7] != 1 {
		t.Fatalf("expected department lookup to be cached, got %d calls", departments.calls[7])
	}
	if priorities.calls[5] != 1 {
		t.Fatalf("expected priority lookup to be cached, got %d calls", priorities.calls[5])
	}
	if statuses.calls != 0 {
		t.Fatalf("expected status repo not to be hit when joined status name is present, got %d calls", statuses.calls)
	}
}

func TestGetTimelineByOrderID_RendersStructureChangeComment(t *testing.T) {
	comment := "Смена структуры: department_id: 1 -> 2"
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
		departmentRepo: &historyDepartmentLookupStub{},
		otdelRepo:      &historyOtdelLookupStub{},
		statusRepo:     &historyStatusLookupStub{},
		priorityRepo:   &historyPriorityLookupStub{},
		logger:         zap.NewNop(),
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
	want := humanizeHistoryStructureComment(comment, nil)
	if timeline[0].Lines[0] != want {
		t.Fatalf("expected structure change line %q, got %q", want, timeline[0].Lines[0])
	}
}

type orderHistoryRepoStub struct {
	events []repositories.OrderHistoryItem
}

func (s *orderHistoryRepoStub) FindByOrderID(context.Context, uint64, uint64, uint64) ([]repositories.OrderHistoryItem, error) {
	return s.events, nil
}

func (s *orderHistoryRepoStub) CreateInTx(context.Context, pgx.Tx, *repositories.OrderHistoryItem) error {
	return nil
}

func (s *orderHistoryRepoStub) IsUserParticipant(context.Context, uint64, uint64) (bool, error) {
	return true, nil
}

func (s *orderHistoryRepoStub) GetOrderHistory(context.Context, uint64, types.Filter) ([]repositories.OrderHistoryItem, error) {
	return s.events, nil
}

type historyUserLookupStub struct {
	users      map[uint64]entities.User
	batchCalls int
	byIDCalls  int
}

func (s *historyUserLookupStub) FindUsersByIDs(context.Context, []uint64) (map[uint64]entities.User, error) {
	s.batchCalls++
	return s.users, nil
}

func (s *historyUserLookupStub) FindUserByID(context.Context, uint64) (*entities.User, error) {
	s.byIDCalls++
	return nil, nil
}

type historyDepartmentLookupStub struct {
	names map[uint64]string
	calls map[uint64]int
}

func (s *historyDepartmentLookupStub) FindDepartment(_ context.Context, id uint64) (*entities.Department, error) {
	if s.calls == nil {
		s.calls = make(map[uint64]int)
	}
	s.calls[id]++
	return &entities.Department{ID: id, Name: s.names[id]}, nil
}

type historyOtdelLookupStub struct {
	names map[uint64]string
	calls map[uint64]int
}

func (s *historyOtdelLookupStub) FindOtdel(_ context.Context, id uint64) (*entities.Otdel, error) {
	if s.calls == nil {
		s.calls = make(map[uint64]int)
	}
	s.calls[id]++
	return &entities.Otdel{ID: id, Name: s.names[id]}, nil
}

type historyStatusLookupStub struct {
	names map[uint64]string
	calls int
}

func (s *historyStatusLookupStub) FindStatus(_ context.Context, id uint64) (*entities.Status, error) {
	s.calls++
	return &entities.Status{ID: id, Name: s.names[id]}, nil
}

type historyPriorityLookupStub struct {
	names map[uint64]string
	calls map[uint64]int
}

func (s *historyPriorityLookupStub) FindByID(_ context.Context, id uint64) (*entities.Priority, error) {
	if s.calls == nil {
		s.calls = make(map[uint64]int)
	}
	s.calls[id]++
	return &entities.Priority{ID: id, Name: s.names[id]}, nil
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}

func historyTime(hour int) time.Time {
	return time.Date(2026, 4, 13, hour, 0, 0, 0, time.UTC)
}
