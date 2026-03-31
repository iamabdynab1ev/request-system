// Файл: internal/sync/handler.go
package sync

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

func trimOptionalString(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

type userSyncConflict struct {
	Field                  string `json:"field"`
	Value                  string `json:"value"`
	IncomingExternalID     string `json:"incoming_external_id"`
	IncomingFIO            string `json:"incoming_fio,omitempty"`
	ConflictUserID         uint64 `json:"conflict_user_id,omitempty"`
	ConflictUserFIO        string `json:"conflict_user_fio,omitempty"`
	ConflictUserExternalID string `json:"conflict_user_external_id,omitempty"`
	ConflictUserStatus     string `json:"conflict_user_status,omitempty"`
}

func (c userSyncConflict) Message() string {
	return fmt.Sprintf(
		"%s '%s' из 1С уже занят активным пользователем id=%d, fio=%s, external_id=%s; incoming_external_id=%s",
		c.Field,
		c.Value,
		c.ConflictUserID,
		c.ConflictUserFIO,
		c.ConflictUserExternalID,
		c.IncomingExternalID,
	)
}

type userSyncValidationError struct {
	Conflicts []userSyncConflict
}

func (e *userSyncValidationError) Error() string {
	if e == nil || len(e.Conflicts) == 0 {
		return "обнаружены конфликты пользователей из 1С"
	}

	if len(e.Conflicts) == 1 {
		return e.Conflicts[0].Message()
	}

	return fmt.Sprintf("обнаружено %d конфликтов пользователей из 1С; первый: %s", len(e.Conflicts), e.Conflicts[0].Message())
}

func (h *DBHandler) ProcessUsers(ctx context.Context, data []dto.User1CDTO) error {
	countTotal := len(data)
	countCreated := 0
	countUpdated := 0
	duplicateEmailAssignments := buildDuplicateEmailAssignments(data)
	incomingPhoneAssignments := buildIncomingPhoneAssignments(data)
	var validationErr *userSyncValidationError

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

			if fio := trimOptionalString(item.Fio); fio != "" {
				entity.Fio = fio
			}

			if item.IsActive != nil {
				if *item.IsActive {
					entity.StatusID = activeStatus.ID
				} else {
					entity.StatusID = inactiveStatus.ID
				}
			}

			incomingActive := entity.StatusID == activeStatus.ID

			var resolvedPosition *entities.Position
			positionFromPayload := false

			positionExternalID := trimOptionalString(item.PositionExternalID)
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

			if depExternalID := trimOptionalString(item.DepartmentExternalID); depExternalID != "" {
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

			if otdelExternalID := trimOptionalString(item.OtdelExternalID); otdelExternalID != "" {
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

			if branchExternalID := trimOptionalString(item.BranchExternalID); branchExternalID != "" {
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

			if officeExternalID := trimOptionalString(item.OfficeExternalID); officeExternalID != "" {
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

			targetUserID := uint64(0)
			if userFound {
				targetUserID = existing.ID
			}

			cleanEmail := trimOptionalString(item.Email)
			if numberedEmail, ok := duplicateEmailAssignments[externalID]; ok {
				h.logger.Warn(
					"Дублирующийся email в выгрузке 1С нормализован нумерацией",
					zap.String("incoming_external_id", externalID),
					zap.String("source_email", cleanEmail),
					zap.String("normalized_email", numberedEmail),
				)
				cleanEmail = numberedEmail
			}
			cleanPhone := trimOptionalString(item.PhoneNumber)
			cleanUsername := trimOptionalString(item.Username)

			if userFound && cleanPhone == "" {
				technicalPhone := buildTechnicalPhoneValue(existing.ID, externalID)
				if entity.PhoneNumber != technicalPhone {
					h.logger.Warn(
						"Телефон пользователя очищен по актуальным данным выгрузки 1С, назначено техническое значение",
						zap.String("incoming_external_id", externalID),
						zap.Uint64("user_id", existing.ID),
						zap.String("old_phone", entity.PhoneNumber),
						zap.String("technical_phone", technicalPhone),
					)
				}
				entity.PhoneNumber = technicalPhone
			}

			conflicts, err := h.collectIncomingContactConflicts(
				ctx,
				tx,
				targetUserID,
				externalID,
				entity.Fio,
				incomingActive,
				cleanEmail,
				cleanPhone,
				cleanUsername,
				incomingPhoneAssignments,
			)
			if err != nil {
				return err
			}
			if len(conflicts) > 0 {
				if validationErr == nil {
					validationErr = &userSyncValidationError{}
				}
				validationErr.Conflicts = append(validationErr.Conflicts, conflicts...)
				continue
			}

			if cleanEmail != "" {
				resolvedEmail, applyResolvedEmail, err := h.resolveIncomingEmailConflict(ctx, tx, targetUserID, externalID, cleanEmail, incomingActive)
				if err != nil {
					return err
				}
				if applyResolvedEmail {
					entity.Email = resolvedEmail
				} else {
					entity.Email = cleanEmail
				}
			}

			normalizePhoneAfterCreate := false
			if cleanPhone != "" {
				resolvedPhone, applyResolvedPhone, deferPhoneNormalization, err := h.resolveIncomingPhoneConflict(ctx, tx, targetUserID, externalID, cleanPhone, incomingActive, incomingPhoneAssignments)
				if err != nil {
					return err
				}
				if applyResolvedPhone {
					entity.PhoneNumber = resolvedPhone
				} else {
					entity.PhoneNumber = cleanPhone
				}
				normalizePhoneAfterCreate = deferPhoneNormalization
			}

			if cleanUsername != "" {
				resolvedUsername, applyResolvedUsername, err := h.resolveIncomingUsernameConflict(ctx, tx, targetUserID, externalID, cleanUsername, incomingActive)
				if err != nil {
					return err
				}
				if applyResolvedUsername {
					entity.Username = resolvedUsername
				} else {
					entity.Username = &cleanUsername
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
				if normalizePhoneAfterCreate {
					entity.PhoneNumber = buildTechnicalPhoneValue(newID, externalID)
					if _, err := tx.Exec(ctx, "UPDATE users SET phone_number = $1, updated_at = NOW() WHERE id = $2", entity.PhoneNumber, newID); err != nil {
						return fmt.Errorf("failed to normalize technical phone for user %d: %w", newID, err)
					}
				}
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

		if validationErr != nil && len(validationErr.Conflicts) > 0 {
			return validationErr
		}

		return nil
	})

	if err != nil {
		var collectedValidationErr *userSyncValidationError
		if errors.As(err, &collectedValidationErr) {
			for _, conflict := range collectedValidationErr.Conflicts {
				h.logger.Error(
					"User sync validation conflict",
					zap.String("field", conflict.Field),
					zap.String("value", conflict.Value),
					zap.String("incoming_external_id", conflict.IncomingExternalID),
					zap.String("incoming_fio", conflict.IncomingFIO),
					zap.Uint64("conflict_user_id", conflict.ConflictUserID),
					zap.String("conflict_user_external_id", conflict.ConflictUserExternalID),
					zap.String("conflict_user_fio", conflict.ConflictUserFIO),
					zap.String("conflict_user_status", conflict.ConflictUserStatus),
				)
			}
		}
		h.logger.Error("Critical user sync error", zap.Error(err))
		return err
	}

	h.logger.Info("User sync finished", zap.Int("incoming", countTotal), zap.Int("created", countCreated), zap.Int("updated", countUpdated))
	return nil
}

// isNotFound проверяет, является ли ошибка сигналом о том, что запись не найдена.
func buildDuplicateEmailAssignments(data []dto.User1CDTO) map[string]string {
	emailToExternalIDs := make(map[string][]string)

	for _, item := range data {
		externalID := strings.TrimSpace(item.ExternalID)
		email := strings.ToLower(trimOptionalString(item.Email))
		if externalID == "" || email == "" {
			continue
		}

		emailToExternalIDs[email] = append(emailToExternalIDs[email], externalID)
	}

	assignments := make(map[string]string)
	for email, externalIDs := range emailToExternalIDs {
		if len(externalIDs) < 2 {
			continue
		}

		sort.Strings(externalIDs)
		for index, externalID := range externalIDs {
			assignments[externalID] = buildNumberedDuplicateEmail(email, index+1, externalID)
		}
	}

	return assignments
}

func buildNumberedDuplicateEmail(email string, ordinal int, externalID string) string {
	atIndex := strings.LastIndex(email, "@")
	if atIndex <= 0 || atIndex >= len(email)-1 {
		return buildTechnicalEmailValue(externalID, 0)
	}

	return fmt.Sprintf("%s.%d@%s", email[:atIndex], ordinal, email[atIndex+1:])
}

func buildIncomingPhoneAssignments(data []dto.User1CDTO) map[string]string {
	assignments := make(map[string]string, len(data))
	for _, item := range data {
		externalID := strings.TrimSpace(item.ExternalID)
		if externalID == "" {
			continue
		}
		assignments[externalID] = trimOptionalString(item.PhoneNumber)
	}

	return assignments
}

func (h *DBHandler) collectIncomingContactConflicts(
	ctx context.Context,
	tx pgx.Tx,
	targetUserID uint64,
	externalID, incomingFIO string,
	incomingActive bool,
	email, phone, username string,
	incomingPhoneAssignments map[string]string,
) ([]userSyncConflict, error) {
	conflicts := make([]userSyncConflict, 0, 3)

	emailConflict, err := h.collectIncomingEmailConflict(ctx, tx, targetUserID, externalID, incomingFIO, incomingActive, email)
	if err != nil {
		return nil, err
	}
	if emailConflict != nil {
		conflicts = append(conflicts, *emailConflict)
	}

	phoneConflict, err := h.collectIncomingPhoneConflict(ctx, tx, targetUserID, externalID, incomingFIO, incomingActive, phone, incomingPhoneAssignments)
	if err != nil {
		return nil, err
	}
	if phoneConflict != nil {
		conflicts = append(conflicts, *phoneConflict)
	}

	usernameConflict, err := h.collectIncomingUsernameConflict(ctx, tx, targetUserID, externalID, incomingFIO, incomingActive, username)
	if err != nil {
		return nil, err
	}
	if usernameConflict != nil {
		conflicts = append(conflicts, *usernameConflict)
	}

	return conflicts, nil
}

func (h *DBHandler) collectIncomingEmailConflict(
	ctx context.Context,
	tx pgx.Tx,
	targetUserID uint64,
	externalID, incomingFIO string,
	incomingActive bool,
	email string,
) (*userSyncConflict, error) {
	if email == "" {
		return nil, nil
	}

	conflictUser, err := h.userRepo.FindAnyUserByEmailInTx(ctx, tx, email)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка поиска конфликта email для пользователя %s: %w", externalID, err)
	}

	if conflictUser.ID == targetUserID || !incomingActive || canReuseContactsFromConflictUser(conflictUser) {
		return nil, nil
	}

	conflict := buildUserSyncConflict("email", email, externalID, incomingFIO, conflictUser)
	return &conflict, nil
}

func (h *DBHandler) collectIncomingPhoneConflict(
	ctx context.Context,
	tx pgx.Tx,
	targetUserID uint64,
	externalID, incomingFIO string,
	incomingActive bool,
	phone string,
	incomingPhoneAssignments map[string]string,
) (*userSyncConflict, error) {
	if phone == "" {
		return nil, nil
	}

	conflictUser, err := h.userRepo.FindAnyUserByPhoneInTx(ctx, tx, phone)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка поиска конфликта телефона для пользователя %s: %w", externalID, err)
	}

	if conflictUser.ID == targetUserID || !incomingActive || canReuseContactsFromConflictUser(conflictUser) || canReusePhoneFromIncomingBatch(conflictUser, incomingPhoneAssignments) {
		return nil, nil
	}

	conflict := buildUserSyncConflict("телефон", phone, externalID, incomingFIO, conflictUser)
	return &conflict, nil
}

func (h *DBHandler) collectIncomingUsernameConflict(
	ctx context.Context,
	tx pgx.Tx,
	targetUserID uint64,
	externalID, incomingFIO string,
	incomingActive bool,
	username string,
) (*userSyncConflict, error) {
	if username == "" {
		return nil, nil
	}

	conflictUser, err := h.userRepo.FindAnyUserByUsernameInTx(ctx, tx, username)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка поиска конфликта username для пользователя %s: %w", externalID, err)
	}

	if conflictUser.ID == targetUserID || !incomingActive || canReuseContactsFromConflictUser(conflictUser) {
		return nil, nil
	}

	conflict := buildUserSyncConflict("username", username, externalID, incomingFIO, conflictUser)
	return &conflict, nil
}

func buildUserSyncConflict(field, value, incomingExternalID, incomingFIO string, conflictUser *entities.User) userSyncConflict {
	conflict := userSyncConflict{
		Field:              field,
		Value:              value,
		IncomingExternalID: incomingExternalID,
		IncomingFIO:        incomingFIO,
	}

	if conflictUser == nil {
		return conflict
	}

	conflict.ConflictUserID = conflictUser.ID
	conflict.ConflictUserFIO = conflictUser.Fio
	conflict.ConflictUserExternalID = stringValue(conflictUser.ExternalID)
	conflict.ConflictUserStatus = conflictUser.StatusCode

	return conflict
}

func (h *DBHandler) resolveIncomingEmailConflict(ctx context.Context, tx pgx.Tx, targetUserID uint64, externalID, email string, incomingActive bool) (string, bool, error) {
	if email == "" {
		return "", false, nil
	}

	conflictUser, err := h.userRepo.FindAnyUserByEmailInTx(ctx, tx, email)
	if err != nil {
		if isNotFound(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("ошибка поиска конфликта email для пользователя %s: %w", externalID, err)
	}

	if conflictUser.ID == targetUserID {
		return "", false, nil
	}

	if !incomingActive {
		technicalEmail := buildTechnicalEmailValue(externalID, targetUserID)
		h.logger.Warn(
			"Конфликт email у неактивного пользователя из 1С, назначено техническое значение",
			zap.String("incoming_external_id", externalID),
			zap.String("conflicting_email", email),
			zap.Uint64("conflict_user_id", conflictUser.ID),
			zap.String("conflict_user_external_id", stringValue(conflictUser.ExternalID)),
			zap.String("technical_email", technicalEmail),
		)
		return technicalEmail, true, nil
	}

	if canReuseContactsFromConflictUser(conflictUser) {
		if err := h.releaseConflictUserEmail(ctx, tx, conflictUser); err != nil {
			return "", false, fmt.Errorf("не удалось освободить email '%s' у пользователя id=%d: %w", email, conflictUser.ID, err)
		}

		h.logger.Warn(
			"Освобожден email неактивного/удаленного пользователя для sync из 1С",
			zap.String("email", email),
			zap.Uint64("released_user_id", conflictUser.ID),
			zap.String("released_user_external_id", stringValue(conflictUser.ExternalID)),
			zap.String("released_user_status", conflictUser.StatusCode),
		)
		return "", false, nil
	}

	return "", false, fmt.Errorf(
		"email '%s' из 1С уже занят активным пользователем id=%d, fio=%s, external_id=%s; incoming_external_id=%s",
		email,
		conflictUser.ID,
		conflictUser.Fio,
		stringValue(conflictUser.ExternalID),
		externalID,
	)
}

func (h *DBHandler) resolveIncomingPhoneConflict(ctx context.Context, tx pgx.Tx, targetUserID uint64, externalID, phone string, incomingActive bool, incomingPhoneAssignments map[string]string) (string, bool, bool, error) {
	if phone == "" {
		return "", false, false, nil
	}

	conflictUser, err := h.userRepo.FindAnyUserByPhoneInTx(ctx, tx, phone)
	if err != nil {
		if isNotFound(err) {
			return "", false, false, nil
		}
		return "", false, false, fmt.Errorf("ошибка поиска конфликта телефона для пользователя %s: %w", externalID, err)
	}

	if conflictUser.ID == targetUserID {
		return "", false, false, nil
	}

	if !incomingActive {
		technicalPhone := buildTechnicalPhoneValue(targetUserID, externalID)
		normalizeAfterCreate := targetUserID == 0
		h.logger.Warn(
			"Конфликт телефона у неактивного пользователя из 1С, назначено техническое значение",
			zap.String("incoming_external_id", externalID),
			zap.String("conflicting_phone", phone),
			zap.Uint64("conflict_user_id", conflictUser.ID),
			zap.String("conflict_user_external_id", stringValue(conflictUser.ExternalID)),
			zap.String("technical_phone", technicalPhone),
		)
		return technicalPhone, true, normalizeAfterCreate, nil
	}

	if canReuseContactsFromConflictUser(conflictUser) {
		if err := h.releaseConflictUserPhone(ctx, tx, conflictUser); err != nil {
			return "", false, false, fmt.Errorf("не удалось освободить телефон '%s' у пользователя id=%d: %w", phone, conflictUser.ID, err)
		}

		h.logger.Warn(
			"Освобожден телефон неактивного/удаленного пользователя для sync из 1С",
			zap.String("phone", phone),
			zap.Uint64("released_user_id", conflictUser.ID),
			zap.String("released_user_external_id", stringValue(conflictUser.ExternalID)),
			zap.String("released_user_status", conflictUser.StatusCode),
		)
		return "", false, false, nil
	}

	if canReusePhoneFromIncomingBatch(conflictUser, incomingPhoneAssignments) {
		if err := h.releaseConflictUserPhone(ctx, tx, conflictUser); err != nil {
			return "", false, false, fmt.Errorf("не удалось освободить телефон '%s' у пользователя id=%d по актуальным данным 1С: %w", phone, conflictUser.ID, err)
		}

		h.logger.Warn(
			"Освобожден телефон пользователя по актуальным данным выгрузки 1С",
			zap.String("phone", phone),
			zap.Uint64("released_user_id", conflictUser.ID),
			zap.String("released_user_external_id", stringValue(conflictUser.ExternalID)),
			zap.String("released_user_status", conflictUser.StatusCode),
		)
		return "", false, false, nil
	}

	return "", false, false, fmt.Errorf(
		"телефон '%s' из 1С уже занят активным пользователем id=%d, fio=%s, external_id=%s; incoming_external_id=%s",
		phone,
		conflictUser.ID,
		conflictUser.Fio,
		stringValue(conflictUser.ExternalID),
		externalID,
	)
}

func (h *DBHandler) resolveIncomingUsernameConflict(ctx context.Context, tx pgx.Tx, targetUserID uint64, externalID, username string, incomingActive bool) (*string, bool, error) {
	if username == "" {
		return nil, false, nil
	}

	conflictUser, err := h.userRepo.FindAnyUserByUsernameInTx(ctx, tx, username)
	if err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("ошибка поиска конфликта username для пользователя %s: %w", externalID, err)
	}

	if conflictUser.ID == targetUserID {
		return nil, false, nil
	}

	if !incomingActive {
		h.logger.Warn(
			"Конфликт username у неактивного пользователя из 1С, username очищен",
			zap.String("incoming_external_id", externalID),
			zap.String("conflicting_username", username),
			zap.Uint64("conflict_user_id", conflictUser.ID),
			zap.String("conflict_user_external_id", stringValue(conflictUser.ExternalID)),
		)
		return nil, true, nil
	}

	if canReuseContactsFromConflictUser(conflictUser) {
		if err := h.userRepo.UpdateUsernameAny(ctx, tx, conflictUser.ID, nil); err != nil {
			return nil, false, fmt.Errorf("не удалось освободить username '%s' у пользователя id=%d: %w", username, conflictUser.ID, err)
		}

		h.logger.Warn(
			"Освобожден username неактивного/удаленного пользователя для sync из 1С",
			zap.String("username", username),
			zap.Uint64("released_user_id", conflictUser.ID),
			zap.String("released_user_external_id", stringValue(conflictUser.ExternalID)),
			zap.String("released_user_status", conflictUser.StatusCode),
		)
		return nil, false, nil
	}

	return nil, false, fmt.Errorf(
		"username '%s' из 1С уже занят активным пользователем id=%d, fio=%s, external_id=%s; incoming_external_id=%s",
		username,
		conflictUser.ID,
		conflictUser.Fio,
		stringValue(conflictUser.ExternalID),
		externalID,
	)
}

func canReuseContactsFromConflictUser(conflictUser *entities.User) bool {
	if conflictUser == nil {
		return false
	}
	if conflictUser.DeletedAt != nil {
		return true
	}
	return !strings.EqualFold(conflictUser.StatusCode, "ACTIVE")
}

func canReusePhoneFromIncomingBatch(conflictUser *entities.User, incomingPhoneAssignments map[string]string) bool {
	if conflictUser == nil || len(incomingPhoneAssignments) == 0 {
		return false
	}

	externalID := stringValue(conflictUser.ExternalID)
	if externalID == "" {
		return false
	}

	incomingPhone, ok := incomingPhoneAssignments[externalID]
	if !ok {
		return false
	}

	effectivePhone := incomingPhone
	if effectivePhone == "" {
		effectivePhone = buildTechnicalPhoneValue(conflictUser.ID, externalID)
	}

	return effectivePhone != conflictUser.PhoneNumber
}

func buildTechnicalEmailValue(externalID string, fallbackUserID uint64) string {
	if externalID != "" {
		return fmt.Sprintf("old_%s@trash.local", externalID)
	}
	return fmt.Sprintf("old_%d@trash.local", fallbackUserID)
}

func buildTechnicalPhoneValue(userID uint64, externalID string) string {
	if userID > 0 {
		return fmt.Sprintf("D_%d", userID)
	}
	if externalID != "" {
		return fmt.Sprintf("N%s", externalID)
	}
	return "D_0"
}

func (h *DBHandler) releaseConflictUserEmail(ctx context.Context, tx pgx.Tx, conflictUser *entities.User) error {
	technicalEmail := buildTechnicalEmailValue(stringValue(conflictUser.ExternalID), conflictUser.ID)
	if _, err := tx.Exec(ctx, "UPDATE users SET email = $1, updated_at = NOW() WHERE id = $2", technicalEmail, conflictUser.ID); err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) releaseConflictUserPhone(ctx context.Context, tx pgx.Tx, conflictUser *entities.User) error {
	technicalPhone := buildTechnicalPhoneValue(conflictUser.ID, stringValue(conflictUser.ExternalID))
	if _, err := tx.Exec(ctx, "UPDATE users SET phone_number = $1, updated_at = NOW() WHERE id = $2", technicalPhone, conflictUser.ID); err != nil {
		return err
	}
	return nil
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

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
		if err != nil {
			return err
		}
		inactiveStatus, err := h.statusRepo.FindByCodeInTx(ctx, tx, "INACTIVE")
		if err != nil {
			return err
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
			if !item.IsActive {
				statusID = inactiveStatus.ID
			}

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
			if !item.IsActive {
				statusID = inactiveStatus.ID
			}

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
			if !item.IsActive {
				statusID = inactiveStatus.ID
			}

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
