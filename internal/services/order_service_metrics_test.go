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

func TestCalculateMetrics_DoesNotOverwriteMetricsWhenCompletedOrderIsClosed(t *testing.T) {
	const (
		completedStatusID = uint64(1)
		closedStatusID    = uint64(2)
		actorID           = uint64(77)
		creatorID         = uint64(88)
		orderID           = uint64(99)
	)

	statusRepo := &statusRepositoryStub{
		codesByID: map[uint64]string{
			completedStatusID: pkgconstants.StatusCompleted,
			closedStatusID:    pkgconstants.StatusClosed,
		},
	}
	service := &OrderService{
		statusRepo: statusRepo,
		logger:     zap.NewNop(),
	}

	createdAt := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	completedAt := createdAt.Add(2 * time.Hour)
	now := completedAt.Add(24 * time.Hour)
	resolutionSeconds := uint64(completedAt.In(time.Local).Sub(createdAt.In(time.Local)).Seconds())
	firstResponseSeconds := uint64(1800)
	isFCR := true
	executorID := actorID

	oldOrder := &entities.Order{
		ID:                       orderID,
		StatusID:                 completedStatusID,
		CreatorID:                creatorID,
		ExecutorID:               &executorID,
		CreatedAt:                createdAt,
		CompletedAt:              &completedAt,
		ResolutionTimeSeconds:    &resolutionSeconds,
		FirstResponseTimeSeconds: &firstResponseSeconds,
		IsFirstContactResolution: &isFCR,
	}
	newOrder := &entities.Order{
		ID:                       orderID,
		StatusID:                 closedStatusID,
		CreatorID:                creatorID,
		ExecutorID:               &executorID,
		CreatedAt:                createdAt,
		CompletedAt:              &completedAt,
		ResolutionTimeSeconds:    &resolutionSeconds,
		FirstResponseTimeSeconds: &firstResponseSeconds,
		IsFirstContactResolution: &isFCR,
	}

	service.calculateMetrics(context.Background(), newOrder, oldOrder, dto.UpdateOrderDTO{}, actorID, now)

	if newOrder.CompletedAt == nil || !newOrder.CompletedAt.Equal(completedAt) {
		t.Fatalf("expected completed_at to stay %v, got %v", completedAt, newOrder.CompletedAt)
	}
	if newOrder.ResolutionTimeSeconds == nil || *newOrder.ResolutionTimeSeconds != resolutionSeconds {
		t.Fatalf("expected resolution time to stay %d, got %v", resolutionSeconds, newOrder.ResolutionTimeSeconds)
	}
	if newOrder.FirstResponseTimeSeconds == nil || *newOrder.FirstResponseTimeSeconds != firstResponseSeconds {
		t.Fatalf("expected first response time to stay %d, got %v", firstResponseSeconds, newOrder.FirstResponseTimeSeconds)
	}
	if newOrder.IsFirstContactResolution == nil || *newOrder.IsFirstContactResolution != isFCR {
		t.Fatalf("expected FCR to stay %v, got %v", isFCR, newOrder.IsFirstContactResolution)
	}
}

func TestCalculateMetrics_ResetResolutionMetricsWhenReturnedToRefinement(t *testing.T) {
	const (
		completedStatusID  = uint64(1)
		refinementStatusID = uint64(2)
		actorID            = uint64(77)
		creatorID          = uint64(88)
		orderID            = uint64(99)
	)

	statusRepo := &statusRepositoryStub{
		codesByID: map[uint64]string{
			completedStatusID:  pkgconstants.StatusCompleted,
			refinementStatusID: pkgconstants.StatusRefinement,
		},
	}
	service := &OrderService{
		statusRepo: statusRepo,
		logger:     zap.NewNop(),
	}

	createdAt := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	completedAt := createdAt.Add(2 * time.Hour)
	now := completedAt.Add(30 * time.Minute)
	resolutionSeconds := uint64(7200)
	firstResponseSeconds := uint64(1800)
	isFCR := true
	executorID := actorID

	oldOrder := &entities.Order{
		ID:                       orderID,
		StatusID:                 completedStatusID,
		CreatorID:                creatorID,
		ExecutorID:               &executorID,
		CreatedAt:                createdAt,
		CompletedAt:              &completedAt,
		ResolutionTimeSeconds:    &resolutionSeconds,
		FirstResponseTimeSeconds: &firstResponseSeconds,
		IsFirstContactResolution: &isFCR,
	}
	newOrder := &entities.Order{
		ID:                       orderID,
		StatusID:                 refinementStatusID,
		CreatorID:                creatorID,
		ExecutorID:               &executorID,
		CreatedAt:                createdAt,
		CompletedAt:              &completedAt,
		ResolutionTimeSeconds:    &resolutionSeconds,
		FirstResponseTimeSeconds: &firstResponseSeconds,
		IsFirstContactResolution: &isFCR,
	}

	service.calculateMetrics(context.Background(), newOrder, oldOrder, dto.UpdateOrderDTO{}, actorID, now)

	if newOrder.CompletedAt != nil {
		t.Fatalf("expected completed_at to be reset, got %v", newOrder.CompletedAt)
	}
	if newOrder.ResolutionTimeSeconds != nil {
		t.Fatalf("expected resolution time to be reset, got %v", newOrder.ResolutionTimeSeconds)
	}
	if newOrder.IsFirstContactResolution != nil {
		t.Fatalf("expected FCR to be reset, got %v", newOrder.IsFirstContactResolution)
	}
	if newOrder.FirstResponseTimeSeconds == nil || *newOrder.FirstResponseTimeSeconds != firstResponseSeconds {
		t.Fatalf("expected first response time to stay %d, got %v", firstResponseSeconds, newOrder.FirstResponseTimeSeconds)
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
