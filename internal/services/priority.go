package services

import (
	"context"
	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

type PriorityServiceInterface interface {
	GetPriorities(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.PriorityDTO], error)
	FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error)
	CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) (*dto.PriorityDTO, error)
	UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) (*dto.PriorityDTO, error)
	DeletePriority(ctx context.Context, id uint64) error
}

type PriorityService struct {
	priorityRepository repositories.PriorityRepositoryInterface
	userRepo           repositories.UserRepositoryInterface
	logger             *zap.Logger
}

func NewPriorityService(
	priorityRepository repositories.PriorityRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) PriorityServiceInterface {
	return &PriorityService{
		priorityRepository: priorityRepository,
		userRepo:           userRepo,
		logger:             logger,
	}
}

func (s *PriorityService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: nil}, nil
}

func (s *PriorityService) GetPriorities(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.PriorityDTO], error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo(authz.PrioritiesView, *authContext) { // <-- ИЗМЕНЕНО
		return nil, apperrors.ErrForbidden
	}

	priorities, total, err := s.priorityRepository.GetPriorities(ctx, limit, offset, search)
	if err != nil {
		return nil, err
	}

	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}

	return &dto.PaginatedResponse[dto.PriorityDTO]{
		List:       priorities,
		Pagination: dto.PaginationObject{TotalCount: total, Page: currentPage, Limit: limit},
	}, nil
}

func (s *PriorityService) FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo(authz.PrioritiesView, *authContext) { // <-- ИЗМЕНЕНО
		return nil, apperrors.ErrForbidden
	}

	return s.priorityRepository.FindPriority(ctx, id)
}

func (s *PriorityService) CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) (*dto.PriorityDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo(authz.PrioritiesCreate, *authContext) { // <-- ИЗМЕНЕНО
		return nil, apperrors.ErrForbidden
	}

	return s.priorityRepository.CreatePriority(ctx, dto)
}

func (s *PriorityService) UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) (*dto.PriorityDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo(authz.PrioritiesUpdate, *authContext) { // <-- ИЗМЕНЕНО
		return nil, apperrors.ErrForbidden
	}

	return s.priorityRepository.UpdatePriority(ctx, id, dto)
}

func (s *PriorityService) DeletePriority(ctx context.Context, id uint64) error {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}

	if !authz.CanDo(authz.PrioritiesDelete, *authContext) { // <-- ИЗМЕНЕНО
		return apperrors.ErrForbidden
	}

	return s.priorityRepository.DeletePriority(ctx, id)
}
