// Файл: internal/services/position_service.go
package services

import (
	"context"
	"time"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type PositionServiceInterface interface {
	Create(ctx context.Context, d dto.CreatePositionDTO) (*dto.PositionResponseDTO, error)
	Update(ctx context.Context, id int, d dto.UpdatePositionDTO) (*dto.PositionResponseDTO, error)
	Delete(ctx context.Context, id int) error
	GetByID(ctx context.Context, id int) (*dto.PositionResponseDTO, error)
	GetAll(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.PositionResponseDTO], error)
}

type PositionService struct {
	repo      repositories.PositionRepositoryInterface
	userRepo  repositories.UserRepositoryInterface
	txManager repositories.TxManagerInterface
	logger    *zap.Logger
}

func NewPositionService(
	repo repositories.PositionRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	txManager repositories.TxManagerInterface,
	logger *zap.Logger,
) PositionServiceInterface {
	return &PositionService{repo: repo, userRepo: userRepo, txManager: txManager, logger: logger}
}

func toPositionResponseDTO(e *entities.Position) *dto.PositionResponseDTO {
	if e == nil {
		return nil
	}
	resp := &dto.PositionResponseDTO{
		ID: uint64(e.Id), Name: e.Name, Level: e.Level, StatusID: e.StatusID,
		CreatedAt: e.CreatedAt.Format(time.RFC3339), UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}
	if e.Code != nil {
		resp.Code = *e.Code
	}
	return resp
}

func (s *PositionService) Create(ctx context.Context, d dto.CreatePositionDTO) (*dto.PositionResponseDTO, error) {
	// ... (логика с проверкой прав, как в OrderTypeService.Create)
	authContext, err := buildAuthzContext(ctx, s.userRepo) // Используем хелпер
	if err != nil {
		return nil, err
	}
	if !authz.CanDo("position:create", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	var newID int
	posEntity := &entities.Position{Name: d.Name, Code: d.Code, Level: d.Level, StatusID: d.StatusID}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		createdID, errTx := s.repo.Create(ctx, tx, posEntity)
		if errTx != nil {
			return errTx
		}
		newID = createdID
		return nil
	})
	if err != nil {
		return nil, err
	}

	created, err := s.repo.FindByID(ctx, newID)
	if err != nil {
		return nil, err
	}
	return toPositionResponseDTO(created), nil
}

func (s *PositionService) Update(ctx context.Context, id int, d dto.UpdatePositionDTO) (*dto.PositionResponseDTO, error) {
	// ... (логика с проверкой прав, как в OrderTypeService.Update)
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo("position:update", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if d.Name != nil {
		existing.Name = *d.Name
	}
	if d.Code != nil {
		existing.Code = d.Code
	}
	if d.Level != nil {
		existing.Level = *d.Level
	}
	if d.StatusID != nil {
		existing.StatusID = *d.StatusID
	}
	now := time.Now()
	existing.UpdatedAt = &now

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.repo.Update(ctx, tx, existing)
	})
	if err != nil {
		return nil, err
	}

	return toPositionResponseDTO(existing), nil
}

func (s *PositionService) Delete(ctx context.Context, id int) error {
	// ... (логика с проверкой прав, как в OrderTypeService.Delete)
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return err
	}
	if !authz.CanDo("position:delete", *authContext) {
		return apperrors.ErrForbidden
	}

	return s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, id)
	})
}

func (s *PositionService) GetByID(ctx context.Context, id int) (*dto.PositionResponseDTO, error) {
	// ... (логика с проверкой прав, как в OrderTypeService.GetByID)
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo("position:view", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	entity, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toPositionResponseDTO(entity), nil
}

func (s *PositionService) GetAll(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.PositionResponseDTO], error) {
	// ... (логика с проверкой прав, как в OrderTypeService.GetAll)
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo("position:view", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	entities, total, err := s.repo.GetAll(ctx, limit, offset, search)
	if err != nil {
		return nil, err
	}

	dtos := make([]dto.PositionResponseDTO, 0, len(entities))
	for _, entity := range entities {
		dtos = append(dtos, *toPositionResponseDTO(entity))
	}

	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}

	return &dto.PaginatedResponse[dto.PositionResponseDTO]{
		List: dtos, Pagination: dto.PaginationObject{TotalCount: total, Page: currentPage, Limit: limit},
	}, nil
}

// Хелпер, который можно вынести в base_service.go
func buildAuthzContext(ctx context.Context, userRepo repositories.UserRepositoryInterface) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return &authz.Context{Actor: actor, Permissions: permissionsMap}, nil
}
