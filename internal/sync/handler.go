// Файл: internal/sync/handler.go
package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

func (h *DBHandler) ProcessUsers(ctx context.Context, data []dto.User1CDTO) error {
	countTotal := len(data)
	countCreated := 0
	countUpdated := 0

	h.logger.Info("Processing users from 1C (partial update mode)", zap.Int("incoming", countTotal))

	err := h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, err := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		if err != nil {
			return err
		}
		inactiveStatus, err := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")
		if err != nil {
			return err
		}

		var defaultRoleIDs []uint64
		for _, roleName := range h.cfg.DefaultRolesFor1CUsers {
			name := strings.TrimSpace(roleName)
			if name == "" {
				continue
			}
			role, err := h.roleRepo.FindByName(ctx, tx, name)
			if err == nil && role != nil {
				defaultRoleIDs = append(defaultRoleIDs, role.ID)
			} else {
				h.logger.Warn("Default role from env not found in DB", zap.String("name", name))
			}
		}

		trimOrEmpty := func(s *string) string {
			if s == nil {
				return ""
			}
			return strings.TrimSpace(*s)
		}

		for _, item := range data {
			externalID := strings.TrimSpace(item.ExternalID)
			if externalID == "" {
				continue
			}

			existing, err := h.userRepo.FindByExternalID(ctx, tx, externalID, sourceSystem1C)
			if err != nil && !isNotFound(err) {
				return fmt.Errorf("DB Error User %s: %w", externalID, err)
			}

			userFound := err == nil && existing != nil && existing.ID != 0
			entity := entities.User{
				Fio:          fmt.Sprintf("1c_user_%s", externalID),
				Email:        fmt.Sprintf("no_email_%s@1c.local", externalID),
				PhoneNumber:  fmt.Sprintf("N%s", externalID),
				StatusID:     activeStatus.ID,
				ExternalID:   stringToPtr(externalID),
				SourceSystem: stringToPtr(sourceSystem1C),
			}
			if userFound {
				entity = *existing
			}

			if fio := trimOrEmpty(item.Fio); fio != "" {
				entity.Fio = fio
			}

			cleanEmail := trimOrEmpty(item.Email)
			if cleanEmail != "" {
				entity.Email = cleanEmail
			}

			cleanPhone := trimOrEmpty(item.PhoneNumber)
			if cleanPhone != "" {
				entity.PhoneNumber = cleanPhone
			}

			if username := trimOrEmpty(item.Username); username != "" {
				entity.Username = &username
			}

			if item.IsActive != nil {
				if *item.IsActive {
					entity.StatusID = activeStatus.ID
				} else {
					entity.StatusID = inactiveStatus.ID
				}
			}

			var resolvedPosition *entities.Position
			positionFromPayload := false

			positionExternalID := trimOrEmpty(item.PositionExternalID)
			if positionExternalID != "" {
				pos, err := h.positionRepo.FindByExternalID(ctx, tx, positionExternalID, sourceSystem1C)
				if err != nil {
					if !isNotFound(err) {
						return fmt.Errorf("DB Error Position %s for user %s: %w", positionExternalID, externalID, err)
					}
					h.logger.Warn("Position from 1C not found, position_id is not changed", zap.String("user_external_id", externalID), zap.String("position_external_id", positionExternalID))
				} else if pos != nil {
					resolvedPosition = pos
					entity.PositionID = &pos.ID
					positionFromPayload = true
				}
			}

			otdelFromPayload := false

			if depExternalID := trimOrEmpty(item.DepartmentExternalID); depExternalID != "" {
				dep, err := h.departmentRepo.FindByExternalID(ctx, tx, depExternalID, sourceSystem1C)
				if err != nil {
					if !isNotFound(err) {
						return fmt.Errorf("DB Error Department %s for user %s: %w", depExternalID, externalID, err)
					}
					h.logger.Warn("Department from 1C not found, department_id is not changed", zap.String("user_external_id", externalID), zap.String("department_external_id", depExternalID))
				} else if dep != nil {
					entity.DepartmentID = &dep.ID
				}
			}

			if otdelExternalID := trimOrEmpty(item.OtdelExternalID); otdelExternalID != "" {
				otdel, err := h.otdelRepo.FindByExternalID(ctx, tx, otdelExternalID, sourceSystem1C)
				if err != nil {
					if !isNotFound(err) {
						return fmt.Errorf("DB Error Otdel %s for user %s: %w", otdelExternalID, externalID, err)
					}
					h.logger.Warn("Otdel from 1C not found, otdel_id is not changed", zap.String("user_external_id", externalID), zap.String("otdel_external_id", otdelExternalID))
				} else if otdel != nil {
					entity.OtdelID = &otdel.ID
					otdelFromPayload = true
				}
			}

			if branchExternalID := trimOrEmpty(item.BranchExternalID); branchExternalID != "" {
				branch, err := h.branchRepo.FindByExternalID(ctx, tx, branchExternalID, sourceSystem1C)
				if err != nil {
					if !isNotFound(err) {
						return fmt.Errorf("DB Error Branch %s for user %s: %w", branchExternalID, externalID, err)
					}
					h.logger.Warn("Branch from 1C not found, branch_id is not changed", zap.String("user_external_id", externalID), zap.String("branch_external_id", branchExternalID))
				} else if branch != nil {
					entity.BranchID = &branch.ID
				}
			}

			if officeExternalID := trimOrEmpty(item.OfficeExternalID); officeExternalID != "" {
				office, err := h.officeRepo.FindByExternalID(ctx, tx, officeExternalID, sourceSystem1C)
				if err != nil {
					if !isNotFound(err) {
						return fmt.Errorf("DB Error Office %s for user %s: %w", officeExternalID, externalID, err)
					}
					h.logger.Warn("Office from 1C not found, office_id is not changed", zap.String("user_external_id", externalID), zap.String("office_external_id", officeExternalID))
				} else if office != nil {
					entity.OfficeID = &office.ID
				}
			}

			if !userFound && resolvedPosition != nil {
				if entity.DepartmentID == nil {
					entity.DepartmentID = resolvedPosition.DepartmentID
				}
				if entity.OtdelID == nil {
					entity.OtdelID = resolvedPosition.OtdelID
				}
				if entity.BranchID == nil {
					entity.BranchID = resolvedPosition.BranchID
				}
				if entity.OfficeID == nil {
					entity.OfficeID = resolvedPosition.OfficeID
				}
			}

			if cleanEmail != "" {
				if userFound {
					_, _ = tx.Exec(ctx, "UPDATE users SET email = 'old_' || id || '@trash.local' WHERE LOWER(email) = LOWER($1) AND id != $2", cleanEmail, existing.ID)
				} else {
					_, _ = tx.Exec(ctx, "UPDATE users SET email = 'old_' || id || '@trash.local' WHERE LOWER(email) = LOWER($1) AND external_id != $2", cleanEmail, externalID)
				}
			}

			if cleanPhone != "" {
				if userFound {
					_, _ = tx.Exec(ctx, "UPDATE users SET phone_number = 'D_' || id::text WHERE phone_number = $1 AND id != $2", cleanPhone, existing.ID)
				} else {
					_, _ = tx.Exec(ctx, "UPDATE users SET phone_number = 'D_' || id::text WHERE phone_number = $1 AND external_id != $2", cleanPhone, externalID)
				}
			}

			var userID uint64
			if userFound {
				_, _ = tx.Exec(ctx, "UPDATE users SET deleted_at = NULL WHERE id = $1", existing.ID)
				if err := h.userRepo.UpdateFromSync(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("Update Error User %s: %w", externalID, err)
				}
				userID = existing.ID
				countUpdated++
			} else {
				entity.Password = "SYNC_USER_NO_PASSWORD"
				newID, err := h.userRepo.CreateFromSync(ctx, tx, entity)
				if err != nil {
					return fmt.Errorf("Create Error User %s: %w", externalID, err)
				}

				userID = newID
				for _, rID := range defaultRoleIDs {
					if _, err := tx.Exec(ctx, "INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", newID, rID); err != nil {
						return err
					}
				}
				countCreated++
			}

			// Keep manual assignments: do not delete links, only ensure links from 1C exist.
			if positionFromPayload && entity.PositionID != nil {
				if _, err := tx.Exec(ctx, "INSERT INTO user_positions (user_id, position_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", userID, *entity.PositionID); err != nil {
					return fmt.Errorf("Sync user_positions failed for user %d: %w", userID, err)
				}
			}

			if otdelFromPayload && entity.OtdelID != nil {
				if _, err := tx.Exec(ctx, "INSERT INTO user_otdels (user_id, otdel_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", userID, *entity.OtdelID); err != nil {
					return fmt.Errorf("Sync user_otdels failed for user %d: %w", userID, err)
				}
			}
		}

		return nil
	})

	if err != nil {
		h.logger.Error("Critical user sync error", zap.Error(err))
		return err
	}

	h.logger.Info("User sync finished", zap.Int("incoming", countTotal), zap.Int("created", countCreated), zap.Int("updated", countUpdated))
	return nil
}

// isNotFound проверяет, является ли ошибка сигналом о том, что запись не найдена.
func isNotFound(err error) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	if errors.Is(err, apperrors.ErrNotFound) {
		return true
	}
	return false
}

// =========================================================================================
// ОБРАБОТЧИКИ
// =========================================================================================

func (h *DBHandler) ProcessDepartments(ctx context.Context, data []dto.Department1CDTO) error {
	countTotal := len(data)
	countCreated := 0
	countUpdated := 0

	err := h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, err := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		if err != nil { return err }
		inactiveStatus, err := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")
		if err != nil { return err }

		for _, item := range data {
			statusID := activeStatus.ID
			if !item.IsActive { statusID = inactiveStatus.ID }

			entity := entities.Department{
				Name:         item.Name,
				StatusID:     statusID,
				ExternalID:   stringToPtr(item.ExternalID),
				SourceSystem: stringToPtr(sourceSystem1C),
			}

			existing, err := h.departmentRepo.FindByExternalID(ctx, tx, item.ExternalID, sourceSystem1C)
			if err != nil && !isNotFound(err) {
				return fmt.Errorf("DB Error Dept %s: %w", item.ExternalID, err)
			}

			if err == nil {
				if err := h.departmentRepo.Update(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("Update Error Dept %s: %w", item.Name, err)
				}
				countUpdated++
			} else {
				if _, err := h.departmentRepo.Create(ctx, tx, entity); err != nil {
					return fmt.Errorf("Create Error Dept %s: %w", item.Name, err)
				}
				countCreated++
			}
		}
		return nil
	})

	if err == nil {
		h.logger.Info("📊 ДЕПАРТАМЕНТЫ", zap.Int("Всего", countTotal), zap.Int("Создано", countCreated), zap.Int("Обновлено", countUpdated))
	}
	return err
}

func (h *DBHandler) ProcessBranches(ctx context.Context, data []dto.Branch1CDTO) error {
	countTotal := len(data)
	countCreated := 0
	countUpdated := 0

	err := h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		inactiveStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")

		for _, item := range data {
			statusID := activeStatus.ID
			if !item.IsActive { statusID = inactiveStatus.ID }

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
			if err != nil && !isNotFound(err) {
				return fmt.Errorf("DB Error Branch %s: %w", item.ExternalID, err)
			}

			if err == nil {
				if err := h.branchRepo.UpdateBranch(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("Update Error Branch %s: %w", item.Name, err)
				}
				countUpdated++
			} else {
				if _, err := h.branchRepo.CreateBranch(ctx, tx, entity); err != nil {
					return fmt.Errorf("Create Error Branch %s: %w", item.Name, err)
				}
				countCreated++
			}
		}
		return nil
	})

	if err == nil {
		h.logger.Info("📊 ФИЛИАЛЫ", zap.Int("Всего", countTotal), zap.Int("Создано", countCreated), zap.Int("Обновлено", countUpdated))
	}
	return err
}

func (h *DBHandler) ProcessOtdels(ctx context.Context, data []dto.Otdel1CDTO) error {
	countTotal := len(data)
	countCreated := 0
	countUpdated := 0

	err := h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		inactiveStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")

		for _, item := range data {
			statusID := activeStatus.ID
			if !item.IsActive { statusID = inactiveStatus.ID }

			var depID, branchID, parentID *uint64
			if item.ParentExternalID != "" {
				if p, _ := h.otdelRepo.FindByExternalID(ctx, tx, item.ParentExternalID, sourceSystem1C); p != nil {
					parentID = &p.ID
				}
			} else if item.DepartmentExternalID != "" {
				if p, _ := h.departmentRepo.FindByExternalID(ctx, tx, item.DepartmentExternalID, sourceSystem1C); p != nil {
					depID = &p.ID
				}
			} else if item.BranchExternalID != "" {
				if p, _ := h.branchRepo.FindByExternalID(ctx, tx, item.BranchExternalID, sourceSystem1C); p != nil {
					branchID = &p.ID
				}
			}

			entity := entities.Otdel{
				Name:          item.Name,
				StatusID:      statusID,
				DepartmentsID: depID,
				BranchID:      branchID,
				ParentID:      parentID,
				ExternalID:    stringToPtr(item.ExternalID),
				SourceSystem:  stringToPtr(sourceSystem1C),
			}

			existing, err := h.otdelRepo.FindByExternalID(ctx, tx, item.ExternalID, sourceSystem1C)
			if err != nil && !isNotFound(err) {
				return fmt.Errorf("DB Error Otdel %s: %w", item.ExternalID, err)
			}

			if err == nil {
				if err := h.otdelRepo.UpdateOtdel(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("Update Error Otdel %s: %w", item.Name, err)
				}
				countUpdated++
			} else {
				if _, err := h.otdelRepo.CreateOtdel(ctx, tx, entity); err != nil {
					return fmt.Errorf("Create Error Otdel %s: %w", item.Name, err)
				}
				countCreated++
			}
		}
		return nil
	})

	if err == nil {
		h.logger.Info("📊 ОТДЕЛЫ", zap.Int("Всего", countTotal), zap.Int("Создано", countCreated), zap.Int("Обновлено", countUpdated))
	}
	return err
}

func (h *DBHandler) ProcessOffices(ctx context.Context, data []dto.Office1CDTO) error {
	countTotal := len(data)
	countCreated := 0
	countUpdated := 0

	err := h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		inactiveStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")

		for _, item := range data {
			statusID := activeStatus.ID
			if !item.IsActive { statusID = inactiveStatus.ID }

			var branchID, parentID *uint64
			if item.ParentExternalID != "" {
				if p, _ := h.officeRepo.FindByExternalID(ctx, tx, item.ParentExternalID, sourceSystem1C); p != nil {
					parentID = &p.ID
				}
			} else if item.BranchExternalID != "" {
				if p, _ := h.branchRepo.FindByExternalID(ctx, tx, item.BranchExternalID, sourceSystem1C); p != nil {
					branchID = &p.ID
				}
			}

			entity := entities.Office{
				Name:         item.Name,
				Address:      item.Address,
				OpenDate:     item.OpenDate,
				StatusID:     statusID,
				BranchID:     branchID,
				ParentID:     parentID,
				ExternalID:   stringToPtr(item.ExternalID),
				SourceSystem: stringToPtr(sourceSystem1C),
			}

			existing, err := h.officeRepo.FindByExternalID(ctx, tx, item.ExternalID, sourceSystem1C)
			if err != nil && !isNotFound(err) {
				return fmt.Errorf("DB Error Office %s: %w", item.ExternalID, err)
			}

			if err == nil {
				if err := h.officeRepo.UpdateOffice(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("Update Error Office %s: %w", item.Name, err)
				}
				countUpdated++
			} else {
				if _, err := h.officeRepo.CreateOffice(ctx, tx, entity); err != nil {
					return fmt.Errorf("Create Error Office %s: %w", item.Name, err)
				}
				countCreated++
			}
		}
		return nil
	})

	if err == nil {
		h.logger.Info("📊 ОФИСЫ", zap.Int("Всего", countTotal), zap.Int("Создано", countCreated), zap.Int("Обновлено", countUpdated))
	}
	return err
}

func (h *DBHandler) ProcessPositions(ctx context.Context, data []dto.Position1CDTO) error {
	countTotal := len(data)
	countCreated := 0
	countUpdated := 0

	err := h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		inactiveStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")

		for _, item := range data {
			var depID, otdelID, branchID, officeID *uint64
			if id := item.DepartmentExternalID; id != nil && *id != "" {
				if p, _ := h.departmentRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C); p != nil { depID = &p.ID }
			}
			if id := item.OtdelExternalID; id != nil && *id != "" {
				if p, _ := h.otdelRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C); p != nil { otdelID = &p.ID }
			}
			if id := item.BranchExternalID; id != nil && *id != "" {
				if p, _ := h.branchRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C); p != nil { branchID = &p.ID }
			}
			if id := item.OfficeExternalID; id != nil && *id != "" {
				if p, _ := h.officeRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C); p != nil { officeID = &p.ID }
			}

			statusID := activeStatus.ID
			if !item.IsActive { statusID = inactiveStatus.ID }

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
			if err != nil && !isNotFound(err) {
				return fmt.Errorf("DB Error Position %s: %w", item.ExternalID, err)
			}

			if err == nil {
				if err := h.positionRepo.Update(ctx, tx, existing.ID, entity); err != nil {
					return fmt.Errorf("Update Error Pos %s: %w", item.Name, err)
				}
				countUpdated++
			} else {
				if _, err := h.positionRepo.Create(ctx, tx, entity); err != nil {
					return fmt.Errorf("Create Error Pos %s: %w", item.Name, err)
				}
				countCreated++
			}
		}
		return nil
	})

	if err == nil {
		h.logger.Info("📊 ДОЛЖНОСТИ", zap.Int("Всего", countTotal), zap.Int("Создано", countCreated), zap.Int("Обновлено", countUpdated))
	}
	return err
}

// =========================================================================================
// ПОЛЬЗОВАТЕЛИ
// =========================================================================================

func (h *DBHandler) processUsersLegacy(ctx context.Context, data []dto.User1CDTO) error {
	countTotal := len(data)
	countCreated := 0
	countUpdated := 0
	
	h.logger.Info("⏳ Обработка пользователей (Режим: СТРОГИЙ)", zap.Int("входящих", countTotal))

	err := h.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		activeStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "ACTIVE")
		inactiveStatus, _ := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")

		// 1. ПРЕДВАРИТЕЛЬНО ЗАГРУЖАЕМ РОЛИ ИЗ .env (один раз на всю транзакцию)
		var defaultRoleIDs []uint64
		for _, roleName := range h.cfg.DefaultRolesFor1CUsers {
			name := strings.TrimSpace(roleName)
			if name == "" {
				continue
			}
			role, err := h.roleRepo.FindByName(ctx, tx, name)
			if err == nil && role != nil {
				defaultRoleIDs = append(defaultRoleIDs, role.ID)
			} else {
				h.logger.Warn("⚠️ Роль из .env не найдена в базе (проверьте написание)", zap.String("name", name))
			}
		}

		// Обработка сотрудников
		for _, item := range data {
			cleanEmail := ""
			if item.Email != nil {
				cleanEmail = strings.TrimSpace(*item.Email)
			}
			cleanPhone := ""
			if item.PhoneNumber != nil {
				cleanPhone = strings.TrimSpace(*item.PhoneNumber)
			}
			fio := ""
			if item.Fio != nil {
				fio = strings.TrimSpace(*item.Fio)
			}
			if item.ExternalID == "" {
				continue
			}

			// Формируем уникальные заглушки для базы
			dbEmail := cleanEmail
			if dbEmail == "" {
				dbEmail = fmt.Sprintf("no_email_%s@1c.local", item.ExternalID)
			}
			dbPhone := cleanPhone
			if dbPhone == "" {
				dbPhone = fmt.Sprintf("N%s", item.ExternalID)
			}
			
			// Проверка должности
			if item.PositionExternalID == nil || strings.TrimSpace(*item.PositionExternalID) == "" {
				continue
			}
			pos, err := h.positionRepo.FindByExternalID(ctx, tx, strings.TrimSpace(*item.PositionExternalID), sourceSystem1C)
			if err != nil {
				continue
			}

			statusID := activeStatus.ID
			if item.IsActive != nil && !*item.IsActive {
				statusID = inactiveStatus.ID
			}

			// Привязка оргструктуры (сверху вниз или по наследованию от должности)
			var depID, otdelID, branchID, officeID *uint64
			if id := item.DepartmentExternalID; id != nil && *id != "" {
				if p, _ := h.departmentRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C); p != nil {
					depID = &p.ID
				}
			}
			if id := item.OtdelExternalID; id != nil && *id != "" {
				if p, _ := h.otdelRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C); p != nil {
					otdelID = &p.ID
				}
			}
			if id := item.BranchExternalID; id != nil && *id != "" {
				if p, _ := h.branchRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C); p != nil {
					branchID = &p.ID
				}
			}
			if id := item.OfficeExternalID; id != nil && *id != "" {
				if p, _ := h.officeRepo.FindByExternalID(ctx, tx, *id, sourceSystem1C); p != nil {
					officeID = &p.ID
				}
			}
			if depID == nil { depID = pos.DepartmentID }
			if otdelID == nil { otdelID = pos.OtdelID }
			if branchID == nil { branchID = pos.BranchID }
			if officeID == nil { officeID = pos.OfficeID }

			existing, err := h.userRepo.FindByExternalID(ctx, tx, item.ExternalID, sourceSystem1C)
			userFound := (err == nil && existing != nil && existing.ID != 0)

			if cleanEmail != "" {
				_, _ = tx.Exec(ctx, "UPDATE users SET email = 'old_' || id || '@trash.local' WHERE LOWER(email) = LOWER($1) AND external_id != $2", cleanEmail, item.ExternalID)
			}
			if !strings.HasPrefix(dbPhone, "N") {
				_, _ = tx.Exec(ctx, "UPDATE users SET phone_number = 'D_' || id::text WHERE phone_number = $1 AND external_id != $2", dbPhone, item.ExternalID)
			}

			var usernamePtr *string
			if item.Username != nil {
				val := strings.TrimSpace(*item.Username)
				if val != "" {
					usernamePtr = &val
				}
			}

			entity := entities.User{
				Fio:          fio,
				Email:        dbEmail,
				PhoneNumber:  dbPhone,
				StatusID:     statusID,
				PositionID:   &pos.ID,
				DepartmentID: depID,
				OtdelID:      otdelID,
				BranchID:     branchID,
				OfficeID:     officeID,
				Username:     usernamePtr,
				ExternalID:   stringToPtr(item.ExternalID),
				SourceSystem: stringToPtr(sourceSystem1C),
			}

			if userFound {
			
				_, _ = tx.Exec(ctx, "UPDATE users SET deleted_at = NULL WHERE id = $1", existing.ID)
				if err := h.userRepo.UpdateFromSync(ctx, tx, existing.ID, entity); err == nil {
					countUpdated++
				}
			} else {
		
				entity.Password = "SYNC_USER_NO_PASSWORD"
				newID, err := h.userRepo.CreateFromSync(ctx, tx, entity)
				if err == nil {
					
					for _, rID := range defaultRoleIDs {
						_, _ = tx.Exec(ctx, "INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", newID, rID)
					}
					countCreated++
				}
			}
		}
		return nil
	})

	if err != nil {
		h.logger.Error("💥 КРИТИЧЕСКАЯ ОШИБКА СИНХРОНИЗАЦИИ", zap.Error(err))
		return err
	}

	h.logger.Info("🏁 СИНХРОНИЗАЦИЯ ПОЛЬЗОВАТЕЛЕЙ ЗАВЕРШЕНА", 
		zap.Int("Пришло", countTotal), 
		zap.Int("Создано", countCreated), 
		zap.Int("Обновлено", countUpdated))
	
	return nil
}
