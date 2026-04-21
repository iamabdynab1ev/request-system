package services

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	pkgconstants "request-system/pkg/constants"
	"request-system/pkg/types"
)

func TestIsOrderResolvedStatus(t *testing.T) {
	if !isOrderResolvedStatus(pkgconstants.StatusCompleted) {
		t.Fatal("expected COMPLETED to be treated as resolved")
	}
	if !isOrderResolvedStatus(pkgconstants.StatusClosed) {
		t.Fatal("expected CLOSED to be treated as resolved")
	}
	if isOrderResolvedStatus(pkgconstants.StatusRejected) {
		t.Fatal("expected REJECTED not to be treated as resolved")
	}
	if isOrderResolvedStatus(pkgconstants.StatusInProgress) {
		t.Fatal("expected IN_PROGRESS not to be treated as resolved")
	}
}

func TestCalculateMetrics_SetsResolutionMetricsForCompletedStatus(t *testing.T) {
	const (
		oldStatusID = uint64(1)
		newStatusID = uint64(2)
		actorID     = uint64(77)
		creatorID   = uint64(88)
		orderID     = uint64(99)
	)

	statusRepo := &statusRepositoryStub{
		codesByID: map[uint64]string{
			oldStatusID: pkgconstants.StatusOpen,
			newStatusID: pkgconstants.StatusCompleted,
		},
	}
	service := &OrderService{
		statusRepo: statusRepo,
		logger:     zap.NewNop(),
	}

	createdAt := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	now := createdAt.Add(2*time.Hour + 15*time.Minute)
	executorID := actorID
	comment := "resolved"

	oldOrder := &entities.Order{
		ID:         orderID,
		StatusID:   oldStatusID,
		CreatorID:  creatorID,
		ExecutorID: &executorID,
		CreatedAt:  createdAt,
	}
	newOrder := &entities.Order{
		ID:         orderID,
		StatusID:   newStatusID,
		CreatorID:  creatorID,
		ExecutorID: &executorID,
		CreatedAt:  createdAt,
	}

	service.calculateMetrics(context.Background(), newOrder, oldOrder, dto.UpdateOrderDTO{Comment: &comment}, actorID, now)

	wantSeconds := uint64(now.In(time.Local).Sub(createdAt.In(time.Local)).Seconds())
	if newOrder.FirstResponseTimeSeconds == nil || *newOrder.FirstResponseTimeSeconds != wantSeconds {
		t.Fatalf("expected first response time %d, got %v", wantSeconds, newOrder.FirstResponseTimeSeconds)
	}
	if newOrder.ResolutionTimeSeconds == nil || *newOrder.ResolutionTimeSeconds != wantSeconds {
		t.Fatalf("expected resolution time %d, got %v", wantSeconds, newOrder.ResolutionTimeSeconds)
	}
	if newOrder.CompletedAt == nil || !newOrder.CompletedAt.Equal(now.In(time.Local)) {
		t.Fatalf("expected completed_at %v, got %v", now.In(time.Local), newOrder.CompletedAt)
	}
	if newOrder.IsFirstContactResolution == nil || !*newOrder.IsFirstContactResolution {
		t.Fatalf("expected FCR=true, got %v", newOrder.IsFirstContactResolution)
	}
}

type statusRepositoryStub struct {
	codesByID map[uint64]string
}

func (s *statusRepositoryStub) GetStatuses(context.Context, types.Filter) ([]dto.StatusDTO, uint64, error) {
	return nil, 0, nil
}

func (s *statusRepositoryStub) FindStatus(_ context.Context, id uint64) (*entities.Status, error) {
	code, ok := s.codesByID[id]
	if !ok {
		return nil, nil
	}

	return &entities.Status{ID: id, Code: &code}, nil
}

func (s *statusRepositoryStub) FindStatusAsDTO(context.Context, uint64) (*dto.StatusDTO, error) {
	return nil, nil
}

func (s *statusRepositoryStub) CreateStatus(context.Context, dto.CreateStatusDTO, string, string) (*dto.StatusDTO, error) {
	return nil, nil
}

func (s *statusRepositoryStub) UpdateStatus(context.Context, uint64, dto.UpdateStatusDTO, *string, *string) (*dto.StatusDTO, error) {
	return nil, nil
}

func (s *statusRepositoryStub) DeleteStatus(context.Context, uint64) error {
	return nil
}

func (s *statusRepositoryStub) FindByCodeInTx(context.Context, pgx.Tx, string) (*entities.Status, error) {
	return nil, nil
}

func (s *statusRepositoryStub) FindByIDInTx(context.Context, pgx.Tx, uint64) (*entities.Status, error) {
	return nil, nil
}

func (s *statusRepositoryStub) FindIDByCode(context.Context, string) (uint64, error) {
	return 0, nil
}

func (s *statusRepositoryStub) FindAll(context.Context) ([]entities.Status, error) {
	return nil, nil
}
