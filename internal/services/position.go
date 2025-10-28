package services

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

// Обновленный интерфейс
type PositionServiceInterface interface {
	Create(ctx context.Context, d dto.CreatePositionDTO) (*dto.PositionResponseDTO, error)
	Update(ctx context.Context, id uint64, d dto.UpdatePositionDTO, rawBody []byte) (*dto.PositionResponseDTO, error)
	Delete(ctx context.Context, id uint64) error
	GetByID(ctx context.Context, id uint64) (*dto.PositionResponseDTO, error)
	GetAll(ctx context.Context, filter types.Filter) (*dto.PaginatedResponse[dto.PositionResponseDTO], error)
	GetTypes(ctx context.Context) ([]dto.PositionTypeDTO, error)
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

// Новая функция-конвертер

// --- ИСПРАВЛЕННЫЙ `toPositionResponseDTO` ---
func toPositionResponseDTO(e *entities.Position) *dto.PositionResponseDTO {
	if e == nil {
		return nil
	}

	// КОММЕНТАРИЙ: Создаем переменную statusID и безопасно извлекаем значение
	// из указателя *uint64. Если указатель nil, значение будет 0.
	var statusID int
	if e.StatusID != nil {
		statusID = int(*e.StatusID)
	}

	return &dto.PositionResponseDTO{
		ID:           uint64(e.ID),
		Name:         e.Name,
		StatusID:     statusID,
		Type:         e.Type,
		CreatedAt:    e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    e.UpdatedAt.Format(time.RFC3339),
		DepartmentID: utils.Uint64PtrToNullInt(e.DepartmentID),
		OtdelID:      utils.Uint64PtrToNullInt(e.OtdelID),
		BranchID:     utils.Uint64PtrToNullInt(e.BranchID),
		OfficeID:     utils.Uint64PtrToNullInt(e.OfficeID),
	}
}

func (s *PositionService) Create(ctx context.Context, d dto.CreatePositionDTO) (*dto.PositionResponseDTO, error) {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PositionsCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	var newID int
	// КОММЕНТАРИЙ: Создаем временную переменную statusID64 типа uint64.
	statusID64 := uint64(d.StatusID)

	posEntity := &entities.Position{
		Name:         d.Name,
		StatusID:     &statusID64,
		Type:         d.Type,
		DepartmentID: utils.NullIntToUint64Ptr(d.DepartmentID),
		OtdelID:      utils.NullIntToUint64Ptr(d.OtdelID),
		BranchID:     utils.NullIntToUint64Ptr(d.BranchID),
		OfficeID:     utils.NullIntToUint64Ptr(d.OfficeID),
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		createdID, errTx := s.repo.Create(ctx, tx, posEntity)
		if errTx != nil {
			return errTx
		}
		newID = createdID
		return nil
	})
	if err != nil {
		s.logger.Error("Ошибка при создании Position", zap.Error(err))
		return nil, err
	}

	created, err := s.repo.FindByID(ctx, nil, uint64(newID))
	if err != nil {
		s.logger.Error("Ошибка при получении Position", zap.Error(err))
		return nil, err
	}
	return toPositionResponseDTO(created), nil
}

func (s *PositionService) GetAll(ctx context.Context, filter types.Filter) (*dto.PaginatedResponse[dto.PositionResponseDTO], error) {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PositionsView, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	entities, total, err := s.repo.GetAll(ctx, filter)
	if err != nil {
		return nil, err
	}

	dtos := make([]dto.PositionResponseDTO, 0, len(entities))
	for _, entity := range entities {
		dtos = append(dtos, *toPositionResponseDTO(entity))
	}

	var currentPage uint64 = 1
	if filter.Limit > 0 {
		currentPage = (uint64(filter.Offset) / uint64(filter.Limit)) + 1
	}

	return &dto.PaginatedResponse[dto.PositionResponseDTO]{
		List: dtos,
		Pagination: dto.PaginationObject{
			TotalCount: total,
			Page:       currentPage,
			Limit:      uint64(filter.Limit),
		},
	}, nil
}

func (s *PositionService) GetTypes(ctx context.Context) ([]dto.PositionTypeDTO, error) {
	typeList := make([]dto.PositionTypeDTO, 0, len(constants.PositionTypeNames))
	for code, name := range constants.PositionTypeNames {
		typeList = append(typeList, dto.PositionTypeDTO{
			Code: string(code),
			Name: name,
		})
	}
	return typeList, nil
}

func (s *PositionService) Update(ctx context.Context, id uint64, patchDTO dto.UpdatePositionDTO, rawBody []byte) (*dto.PositionResponseDTO, error) {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PositionsUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	existing, err := s.repo.FindByID(ctx, nil, id)
	if err != nil {
		return nil, err
	}

	if err := utils.ApplyPatchFinal(existing, patchDTO, rawBody); err != nil {
		s.logger.Error("Ошибка применения патча для Position", zap.Error(err))
		return nil, err
	}

	// КОММЕНТАРИЙ: В `entities.Position` поле `UpdatedAt` - это `time.Time`, а не `*time.Time`.
	// Поэтому мы присваиваем `time.Now()` напрямую.
	existing.UpdatedAt = time.Now()

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.repo.Update(ctx, tx, existing)
	})
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.FindByID(ctx, nil, id)
	if err != nil {
		return nil, err
	}

	return toPositionResponseDTO(updated), nil
}

func (s *PositionService) GetByID(ctx context.Context, id uint64) (*dto.PositionResponseDTO, error) {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PositionsView, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	entity, err := s.repo.FindByID(ctx, nil, id)
	if err != nil {
		return nil, err
	}
	return toPositionResponseDTO(entity), nil
}

func (s *PositionService) Delete(ctx context.Context, id uint64) error {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.PositionsDelete, *authContext) {
		return apperrors.ErrForbidden
	}

	return s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, int(id))
	})
}

func buildAuthzContext(ctx context.Context, userRepo repositories.UserRepositoryInterface) (*authz.Context, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return &authz.Context{Actor: actor, Permissions: permissionsMap}, nil
}
