// Файл: internal/sync/handler.go
package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/config"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

const (
	sourceSystem1C = "1c"
)

type HandlerInterface interface {
	ProcessDepartments(ctx context.Context, departments []dto.Department1CDTO) error
	ProcessBranches(ctx context.Context, branches []dto.Branch1CDTO) error
	ProcessOtdels(ctx context.Context, otdels []dto.Otdel1CDTO) error
	ProcessOffices(ctx context.Context, offices []dto.Office1CDTO) error
	ProcessPositions(ctx context.Context, positions []dto.Position1CDTO) error
	ProcessUsers(ctx context.Context, users []dto.User1CDTO) error
}

type DBHandler struct {
	txManager      repositories.TxManagerInterface
	branchRepo     repositories.BranchRepositoryInterface
	officeRepo     repositories.OfficeRepositoryInterface
	statusRepo     repositories.StatusRepositoryInterface
	departmentRepo repositories.DepartmentRepositoryInterface
	otdelRepo      repositories.OtdelRepositoryInterface
	positionRepo   repositories.PositionRepositoryInterface
	userRepo       repositories.UserRepositoryInterface
	roleRepo       repositories.RoleRepositoryInterface
	cfg            *config.IntegrationsConfig
	logger         *zap.Logger
}

func NewDBHandler(
	txManager repositories.TxManagerInterface,
	branchRepo repositories.BranchRepositoryInterface,
	officeRepo repositories.OfficeRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	departmentRepo repositories.DepartmentRepositoryInterface,
	otdelRepo repositories.OtdelRepositoryInterface,
	positionRepo repositories.PositionRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	roleRepo repositories.RoleRepositoryInterface,
	cfg *config.IntegrationsConfig,
	logger *zap.Logger,
) HandlerInterface {
	return &DBHandler{
		txManager:      txManager,
		branchRepo:     branchRepo,
		officeRepo:     officeRepo,
		statusRepo:     statusRepo,
		departmentRepo: departmentRepo,
		otdelRepo:      otdelRepo,
		positionRepo:   positionRepo,
		userRepo:       userRepo,
		roleRepo:       roleRepo,
		cfg:            cfg,
		logger:         logger,
	}
}

func stringToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (h *DBHandler) ProcessDepartments(ctx context.Context, data []dto.Department1CDTO) error {
	return h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, err := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		if err != nil {
			return fmt.Errorf("статус 'ACTIVE' не найден: %w", err)
		}
		inactiveStatus, err := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")
		if err != nil {
			return fmt.Errorf("статус 'INACTIVE' не найден: %w", err)
		}

		for _, item := range data {
			statusID := activeStatus.ID
			if !item.IsActive {
				statusID = inactiveStatus.ID
			}

			entity := entities.Department{
				Name:         item.Name,
				StatusID:     statusID,
				ExternalID:   stringToPtr(item.ExternalID),
				SourceSystem: stringToPtr(sourceSystem1C),
			}

			existing, err := h.departmentRepo.FindByExternalID(ctx, tx, item.ExternalID, sourceSystem1C)
			if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
				return fmt.Errorf("ошибка поиска департамента '%s': %w", item.ExternalID, err)
			}

			if existing != nil {
				if err := h.departmentRepo.Update(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("ошибка обновления департамента '%s': %w", item.Name, err)
				}
			} else {
				if _, err := h.departmentRepo.Create(ctx, tx, entity); err != nil {
					return fmt.Errorf("ошибка создания департамента '%s': %w", item.Name, err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) ProcessBranches(ctx context.Context, data []dto.Branch1CDTO) error {
	return h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		inactiveStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")
		if activeStatus == nil || inactiveStatus == nil {
			return fmt.Errorf("не найдены статусы ACTIVE/INACTIVE")
		}

		for _, item := range data {
			statusID := activeStatus.ID
			if !item.IsActive {
				statusID = inactiveStatus.ID
			}

			// ИЗМЕНЕНИЕ: Используем хелперы для преобразования в указатели
			entity := entities.Branch{
				Name:         item.Name,
				ShortName:    item.ShortName,
				Address:      utils.StringToPtr(item.Address),
				PhoneNumber:  utils.StringToPtr(item.PhoneNumber),
				Email:        utils.StringToPtr(item.Email),
				EmailIndex:   utils.StringToPtr(item.EmailIndex),
				OpenDate:     utils.TimeToPtr(item.OpenDate),
				StatusID:     statusID,
				ExternalID:   utils.StringToPtr(item.ExternalID),
				SourceSystem: utils.StringToPtr(sourceSystem1C),
			}

			existing, err := h.branchRepo.FindByExternalID(ctx, tx, item.ExternalID, sourceSystem1C)
			if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
				return fmt.Errorf("ошибка поиска филиала '%s': %w", item.ExternalID, err)
			}
			if existing != nil {
				if err := h.branchRepo.UpdateBranch(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("ошибка обновления филиала '%s': %w", item.Name, err)
				}
			} else {
				if _, err := h.branchRepo.CreateBranch(ctx, tx, entity); err != nil {
					return fmt.Errorf("ошибка создания филиала '%s': %w", item.Name, err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) ProcessOtdels(ctx context.Context, data []dto.Otdel1CDTO) error {
	return h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		inactiveStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")
		if activeStatus == nil || inactiveStatus == nil {
			return fmt.Errorf("не найдены статусы ACTIVE/INACTIVE")
		}

		for _, item := range data {
			if item.DepartmentExternalID == "" {
				return fmt.Errorf("для отдела '%s' не указан обязательный ID департамента (departmentExternalId). Синхронизация отделов отменена", item.Name)
			}
			parentDep, err := h.departmentRepo.FindByExternalID(ctx, tx, item.DepartmentExternalID, sourceSystem1C)
			if err != nil {
				return fmt.Errorf("для отдела '%s' указан несуществующий департамент (external_id: '%s'). Убедитесь, что справочник департаментов синхронизирован первым. Синхронизация отделов отменена",
					item.Name, item.DepartmentExternalID)
			}

			statusID := activeStatus.ID
			if !item.IsActive {
				statusID = inactiveStatus.ID
			}

			entity := entities.Otdel{
				Name:          item.Name,
				StatusID:      statusID,
				DepartmentsID: parentDep.ID,
				ExternalID:    stringToPtr(item.ExternalID),
				SourceSystem:  stringToPtr(sourceSystem1C),
			}

			existing, err := h.otdelRepo.FindByExternalID(ctx, tx, item.ExternalID, sourceSystem1C)
			if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
				return fmt.Errorf("ошибка поиска отдела '%s': %w", item.ExternalID, err)
			}

			if existing != nil {
				if err := h.otdelRepo.UpdateOtdel(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("ошибка обновления отдела '%s': %w", item.Name, err)
				}
			} else {
				if _, err := h.otdelRepo.CreateOtdel(ctx, tx, entity); err != nil {
					return fmt.Errorf("ошибка создания отдела '%s': %w", item.Name, err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) ProcessOffices(ctx context.Context, data []dto.Office1CDTO) error {
	return h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		inactiveStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")
		if activeStatus == nil || inactiveStatus == nil {
			return fmt.Errorf("не найдены статусы ACTIVE/INACTIVE")
		}

		for _, item := range data {
			if item.BranchExternalID == "" {
				return fmt.Errorf("для офиса '%s' не указан обязательный ID филиала (branchExternalId). Синхронизация офисов отменена", item.Name)
			}
			parentBranch, err := h.branchRepo.FindByExternalID(ctx, tx, item.BranchExternalID, sourceSystem1C)
			if err != nil {
				return fmt.Errorf("для офиса '%s' указан несуществующий филиал (external_id: '%s'). Убедитесь, что справочник филиалов синхронизирован первым. Синхронизация офисов отменена",
					item.Name, item.BranchExternalID)
			}

			statusID := activeStatus.ID
			if !item.IsActive {
				statusID = inactiveStatus.ID
			}

			entity := entities.Office{
				Name:         item.Name,
				Address:      item.Address,
				OpenDate:     item.OpenDate,
				StatusID:     statusID,
				BranchID:     parentBranch.ID,
				ExternalID:   stringToPtr(item.ExternalID),
				SourceSystem: stringToPtr(sourceSystem1C),
			}

			existing, err := h.officeRepo.FindByExternalID(ctx, tx, item.ExternalID, sourceSystem1C)
			if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
				return fmt.Errorf("ошибка поиска офиса '%s': %w", item.ExternalID, err)
			}

			if existing != nil {
				if err := h.officeRepo.UpdateOffice(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("ошибка обновления офиса '%s': %w", item.Name, err)
				}
			} else {
				if _, err := h.officeRepo.CreateOffice(ctx, tx, entity); err != nil {
					return fmt.Errorf("ошибка создания офиса '%s': %w", item.Name, err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) ProcessPositions(ctx context.Context, data []dto.Position1CDTO) error {
	return h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		inactiveStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")
		if activeStatus == nil || inactiveStatus == nil {
			return fmt.Errorf("не найдены статусы ACTIVE/INACTIVE")
		}

		for _, item := range data {
			var depID, otdelID, branchID, officeID *uint64

			if id := item.DepartmentExternalID; id != nil && *id != "" {
				parent, err := h.departmentRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C)
				if err != nil {
					return fmt.Errorf("для должности '%s' указан несуществующий департамент (external_id: '%s')", item.Name, *id)
				}
				depID = &parent.ID
			} else if id := item.OtdelExternalID; id != nil && *id != "" {
				parent, err := h.otdelRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C)
				if err != nil {
					return fmt.Errorf("для должности '%s' указан несуществующий отдел (external_id: '%s')", item.Name, *id)
				}
				otdelID = &parent.ID
			} else if id := item.BranchExternalID; id != nil && *id != "" {
				parent, err := h.branchRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C)
				if err != nil {
					return fmt.Errorf("для должности '%s' указан несуществующий филиал (external_id: '%s')", item.Name, *id)
				}
				branchID = &parent.ID
			} else if id := item.OfficeExternalID; id != nil && *id != "" {
				parent, err := h.officeRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C)
				if err != nil {
					return fmt.Errorf("для должности '%s' указан несуществующий офис (external_id: '%s')", item.Name, *id)
				}
				officeID = &parent.ID
			}

			statusID := activeStatus.ID
			if !item.IsActive {
				statusID = inactiveStatus.ID
			}

			entity := entities.Position{
				Name:         item.Name,
				StatusID:     &statusID,
				Type:         item.PositionType,
				DepartmentID: depID,
				OtdelID:      otdelID,
				BranchID:     branchID,
				OfficeID:     officeID,
				ExternalID:   stringToPtr(item.ExternalID),
				SourceSystem: stringToPtr(sourceSystem1C),
			}

			existing, err := h.positionRepo.FindByExternalID(ctx, tx, item.ExternalID, sourceSystem1C)
			if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
				return fmt.Errorf("ошибка поиска должности '%s': %w", item.ExternalID, err)
			}

			if existing != nil {
				if err := h.positionRepo.Update(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("ошибка обновления должности '%s': %w", item.Name, err)
				}
			} else {
				if _, err := h.positionRepo.Create(ctx, tx, entity); err != nil {
					return fmt.Errorf("ошибка создания должности '%s': %w", item.Name, err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) ProcessUsers(ctx context.Context, data []dto.User1CDTO) error {
	return h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, err := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		if err != nil {
			return fmt.Errorf("статус 'ACTIVE' не найден: %w", err)
		}
		inactiveStatus, err := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")
		if err != nil {
			return fmt.Errorf("статус 'INACTIVE' не найден: %w", err)
		}

		var defaultRoleIDs []uint64
		for _, roleName := range h.cfg.DefaultRolesFor1CUsers {
			if roleName == "" {
				continue
			}
			role, err := h.roleRepo.FindByName(ctx, tx, roleName)
			if err != nil {
				h.logger.Warn("Не удалось найти роль по умолчанию из конфигурации. Она не будет назначена.",
					zap.String("roleName", roleName), zap.Error(err))
			} else {
				defaultRoleIDs = append(defaultRoleIDs, role.ID)
			}
		}
		if len(h.cfg.DefaultRolesFor1CUsers) > 0 && len(defaultRoleIDs) == 0 {
			h.logger.Error("Ни одна из ролей по умолчанию, указанных в .env, не найдена в базе. Новые пользователи будут созданы без ролей.")
		}

		for _, item := range data {
			if item.PositionExternalID == "" {
				return fmt.Errorf("для пользователя '%s' (external_id: '%s') не указан обязательный ID должности (positionExternalId). Синхронизация пользователей отменена", item.Fio, item.ExternalID)
			}
			pos, err := h.positionRepo.FindByExternalID(ctx, tx, item.PositionExternalID, sourceSystem1C)
			if err != nil {
				return fmt.Errorf("для пользователя '%s' (external_id: '%s') указана несуществующая должность (positionExternalId: '%s'). Убедитесь, что справочник должностей синхронизирован первым. Синхронизация пользователей отменена",
					item.Fio, item.ExternalID, item.PositionExternalID)
			}

			statusID := activeStatus.ID
			if !item.IsActive {
				statusID = inactiveStatus.ID
			}

			var depID, otdelID, branchID, officeID *uint64

			if id := item.DepartmentExternalID; id != nil && *id != "" {
				parent, err := h.departmentRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C)
				if err != nil {
					return fmt.Errorf("для пользователя '%s' указан несуществующий департамент (external_id: '%s')", item.Fio, *id)
				}
				depID = &parent.ID
			}
			if id := item.OtdelExternalID; id != nil && *id != "" {
				parent, err := h.otdelRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C)
				if err != nil {
					return fmt.Errorf("для пользователя '%s' указан несуществующий отдел (external_id: '%s')", item.Fio, *id)
				}
				otdelID = &parent.ID
			}
			if id := item.BranchExternalID; id != nil && *id != "" {
				parent, err := h.branchRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C)
				if err != nil {
					return fmt.Errorf("для пользователя '%s' указан несуществующий филиал (external_id: '%s')", item.Fio, *id)
				}
				branchID = &parent.ID
			}
			if id := item.OfficeExternalID; id != nil && *id != "" {
				parent, err := h.officeRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C)
				if err != nil {
					return fmt.Errorf("для пользователя '%s' указан несуществующий офис (external_id: '%s')", item.Fio, *id)
				}
				officeID = &parent.ID
			}

			if depID == nil {
				depID = pos.DepartmentID
			}
			if otdelID == nil {
				otdelID = pos.OtdelID
			}
			if branchID == nil {
				branchID = pos.BranchID
			}
			if officeID == nil {
				officeID = pos.OfficeID
			}

			entity := entities.User{
				Fio:          item.Fio,
				Email:        item.Email,
				PhoneNumber:  item.PhoneNumber,
				StatusID:     statusID,
				PositionID:   &pos.ID,
				DepartmentID: depID,
				OtdelID:      otdelID,
				BranchID:     branchID,
				OfficeID:     officeID,
				ExternalID:   stringToPtr(item.ExternalID),
				SourceSystem: stringToPtr(sourceSystem1C),
			}

			existing, err := h.userRepo.FindByExternalID(ctx, tx, item.ExternalID, sourceSystem1C)
			if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
				return fmt.Errorf("ошибка поиска пользователя '%s': %w", item.ExternalID, err)
			}

			if existing != nil {
				if err := h.userRepo.UpdateFromSync(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("ошибка обновления пользователя '%s': %w", item.Fio, err)
				}
			} else {
				entity.Password = "SYNC_USER_NO_PASSWORD"
				newID, err := h.userRepo.CreateFromSync(ctx, tx, entity)
				if err != nil {
					return fmt.Errorf("ошибка создания пользователя '%s': %w", item.Fio, err)
				}

				if len(defaultRoleIDs) > 0 {
					if err := h.userRepo.SyncUserRoles(ctx, tx, newID, defaultRoleIDs); err != nil {
						// Ошибка назначения ролей не должна "валить" всю транзакцию,
						// поэтому мы ее только логируем, но не возвращаем.
						h.logger.Error("Не удалось назначить роли по умолчанию новому пользователю",
							zap.Uint64("userID", newID), zap.Error(err))
					}
				}
			}
		}
		return nil
	})
}
