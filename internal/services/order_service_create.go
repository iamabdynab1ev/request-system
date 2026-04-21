package services

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
)

func (s *OrderService) CreateOrder(ctx context.Context, createDTO dto.CreateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx, 0)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	if err := s.validateCreateFieldPermissions(authCtx, createDTO, file); err != nil {
		return nil, err
	}
	if err := s.validateOrderRules(ctx, createDTO); err != nil {
		return nil, err
	}

	hasDepartment := createDTO.DepartmentID != nil
	hasBranch := createDTO.BranchID != nil
	hasOtdel := createDTO.OtdelID != nil
	hasOffice := createDTO.OfficeID != nil
	if !hasDepartment && !hasBranch && !hasOtdel && !hasOffice {
		return nil, apperrors.NewHttpError(
			http.StatusBadRequest,
			"Необходимо указать хотя бы одно подразделение (департамент, филиал, отдел или офис ЦБО).",
			nil,
			nil,
		)
	}

	var createdID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()

		orderCtx := buildOrderRoutingContext(
			createDTO.OrderTypeID,
			createDTO.DepartmentID,
			createDTO.OtdelID,
			createDTO.BranchID,
			createDTO.OfficeID,
		)

		routingResult, err := s.ruleEngine.ResolveExecutor(ctx, tx, orderCtx, createDTO.ExecutorID)
		if err != nil {
			return err
		}
		if routingResult.Executor.ID == 0 {
			return apperrors.NewHttpError(
				http.StatusBadRequest,
				"Не найден руководитель для выбранной структуры. Настройте правила маршрутизации или укажите исполнителя вручную.",
				nil,
				nil,
			)
		}

		status, err := s.statusRepo.FindByCodeInTx(ctx, tx, "OPEN")
		if err != nil {
			return apperrors.ErrInternalServer
		}

		orderEntity := &entities.Order{
			Name:            createDTO.Name,
			Address:         createDTO.Address,
			OrderTypeID:     createDTO.OrderTypeID,
			DepartmentID:    createDTO.DepartmentID,
			OtdelID:         createDTO.OtdelID,
			BranchID:        createDTO.BranchID,
			OfficeID:        createDTO.OfficeID,
			PriorityID:      createDTO.PriorityID,
			EquipmentID:     createDTO.EquipmentID,
			EquipmentTypeID: createDTO.EquipmentTypeID,
			StatusID:        uint64(status.ID),
			CreatorID:       authCtx.Actor.ID,
			ExecutorID:      &routingResult.Executor.ID,
			Duration:        createDTO.Duration,
		}

		newID, err := s.orderRepo.Create(ctx, tx, orderEntity)
		if err != nil {
			return err
		}
		createdID = newID
		orderEntity.ID = newID

		commentText := ""
		if createDTO.Comment != nil {
			commentText = *createDTO.Comment
		}

		if err := s.logHistoryEvent(ctx, tx, orderEntity.ID, authCtx.Actor, "CREATE", &orderEntity.Name, nil, nil, txID, *orderEntity); err != nil {
			return err
		}
		if commentText != "" {
			if err := s.logHistoryEvent(ctx, tx, orderEntity.ID, authCtx.Actor, "COMMENT", nil, nil, &commentText, txID, *orderEntity); err != nil {
				return err
			}
		}

		delegationText := "Назначено на: " + routingResult.Executor.Fio
		executorIDText := fmt.Sprintf("%d", routingResult.Executor.ID)
		if err := s.logHistoryEvent(ctx, tx, orderEntity.ID, authCtx.Actor, "DELEGATION", &executorIDText, nil, &delegationText, txID, *orderEntity); err != nil {
			return err
		}

		statusIDText := fmt.Sprintf("%d", status.ID)
		if err := s.logHistoryEvent(ctx, tx, orderEntity.ID, authCtx.Actor, "STATUS_CHANGE", &statusIDText, nil, nil, txID, *orderEntity); err != nil {
			return err
		}

		if file != nil {
			if _, err := s.attachFileToOrderInTx(ctx, tx, orderEntity.ID, authCtx.Actor.ID, file, &txID, orderEntity); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.invalidateDashboardCache(ctx, true, true)
	return s.FindOrderByID(ctx, createdID)
}
