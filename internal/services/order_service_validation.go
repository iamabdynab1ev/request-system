package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

func buildOrderRoutingContext(orderTypeID, departmentID, otdelID, branchID, officeID *uint64) OrderContext {
	return OrderContext{
		OrderTypeID:  utils.SafeDeref(orderTypeID),
		DepartmentID: utils.SafeDeref(departmentID),
		OtdelID:      otdelID,
		BranchID:     branchID,
		OfficeID:     officeID,
	}
}

func (s *OrderService) validateUpdateCommentRequirement(ctx context.Context, currentOrder *entities.Order, updateDTO dto.UpdateOrderDTO) error {
	orderTypeCode, _ := s.orderTypeRepo.FindCodeByID(ctx, *currentOrder.OrderTypeID)
	if orderTypeCode == "EQUIPMENT" {
		return nil
	}
	if updateDTO.Comment == nil || strings.TrimSpace(*updateDTO.Comment) == "" {
		return apperrors.NewBadRequestError("Для сохранения изменений необходимо добавить комментарий с описанием действий.")
	}
	return nil
}

func (s *OrderService) applyUpdateExecutorRouting(
	ctx context.Context,
	tx pgx.Tx,
	orderID uint64,
	currentOrder *entities.Order,
	updated *entities.Order,
	updateDTO dto.UpdateOrderDTO,
	explicitFields map[string]interface{},
	authCtx *authz.Context,
) (bool, error) {
	structureChanged := utils.DiffPtr(currentOrder.DepartmentID, updated.DepartmentID) ||
		utils.DiffPtr(currentOrder.OtdelID, updated.OtdelID) ||
		utils.DiffPtr(currentOrder.BranchID, updated.BranchID) ||
		utils.DiffPtr(currentOrder.OfficeID, updated.OfficeID)

	explicitExecutorSelected := false
	if rawExecutor, exists := explicitFields["executor_id"]; exists && rawExecutor != nil {
		explicitExecutorSelected = true
	}

	routingChanged := false
	orderCtx := buildOrderRoutingContext(
		updated.OrderTypeID,
		updated.DepartmentID,
		updated.OtdelID,
		updated.BranchID,
		updated.OfficeID,
	)

	if explicitExecutorSelected {
		if !authz.CanDo(authz.OrdersUpdateExecutorID, *authCtx) {
			return false, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав назначать исполнителя вручную.", nil, nil)
		}

		routingResult, err := s.ruleEngine.ResolveExecutor(ctx, tx, orderCtx, updateDTO.ExecutorID)
		if err != nil {
			return false, err
		}
		updated.ExecutorID = &routingResult.Executor.ID
		routingChanged = true
	}

	if structureChanged {
		s.logger.Info("Изменение структуры -> поиск исполнителя", zap.Uint64("order_id", orderID))
		if !explicitExecutorSelected {
			res, err := s.ruleEngine.ResolveExecutor(ctx, tx, orderCtx, nil)
			if err != nil {
				return false, s.wrapExecutorResolutionError(err, updated)
			}
			updated.ExecutorID = &res.Executor.ID
		}
		routingChanged = true
	}

	return routingChanged, nil
}

func (s *OrderService) wrapExecutorResolutionError(err error, order *entities.Order) error {
	var httpErr *apperrors.HttpError
	if errors.As(err, &httpErr) && httpErr.Code == http.StatusBadRequest {
		return apperrors.NewBadRequestError(buildMissingResponsibleError(order))
	}
	return err
}

func (s *OrderService) validateUpdateFieldPermissions(authCtx *authz.Context, explicitFields map[string]interface{}, file *multipart.FileHeader) error {
	for fieldName, spec := range orderUpdateFieldPermissions {
		if _, exists := explicitFields[fieldName]; !exists {
			continue
		}
		if authz.CanDo(spec.Permission, *authCtx) {
			continue
		}
		return apperrors.NewHttpError(
			http.StatusForbidden,
			fmt.Sprintf("У вас нет прав изменять поле «%s».", spec.Label),
			nil,
			map[string]interface{}{"field": fieldName, "permission": spec.Permission},
		)
	}

	if file != nil && !authz.CanDo(authz.OrdersUpdateFile, *authCtx) {
		return apperrors.NewHttpError(
			http.StatusForbidden,
			"У вас нет прав добавлять файл к заявке.",
			nil,
			map[string]interface{}{"field": "file", "permission": authz.OrdersUpdateFile},
		)
	}

	return nil
}

func (s *OrderService) validateCreateFieldPermissions(authCtx *authz.Context, createDTO dto.CreateOrderDTO, file *multipart.FileHeader) error {
	presentFields := map[string]bool{
		"name":              strings.TrimSpace(createDTO.Name) != "",
		"address":           createDTO.Address != nil,
		"department_id":     createDTO.DepartmentID != nil,
		"otdel_id":          createDTO.OtdelID != nil,
		"branch_id":         createDTO.BranchID != nil,
		"office_id":         createDTO.OfficeID != nil,
		"equipment_id":      createDTO.EquipmentID != nil,
		"equipment_type_id": createDTO.EquipmentTypeID != nil,
		"executor_id":       createDTO.ExecutorID != nil,
		"priority_id":       createDTO.PriorityID != nil,
		"duration":          createDTO.Duration != nil,
		"comment":           createDTO.Comment != nil,
	}

	for fieldName, spec := range orderCreateFieldPermissions {
		if !presentFields[fieldName] {
			continue
		}
		if authz.CanDo(spec.Permission, *authCtx) {
			continue
		}

		return apperrors.NewHttpError(
			http.StatusForbidden,
			fmt.Sprintf("У вас нет прав заполнять поле «%s» при создании заявки.", spec.Label),
			nil,
			map[string]interface{}{"field": fieldName, "permission": spec.Permission},
		)
	}

	if file != nil && !authz.CanDo(authz.OrdersCreateFile, *authCtx) {
		return apperrors.NewHttpError(
			http.StatusForbidden,
			"У вас нет прав добавлять файл при создании заявки.",
			nil,
			map[string]interface{}{"field": "file", "permission": authz.OrdersCreateFile},
		)
	}

	return nil
}

func buildMissingResponsibleError(order *entities.Order) string {
	switch {
	case order.DepartmentID != nil:
		return "В выбранном департаменте не найден руководитель или заместитель. Выберите исполнителя вручную или настройте маршрутизацию."
	case order.OtdelID != nil:
		return "В выбранном отделе не найден руководитель или заместитель. Выберите исполнителя вручную или настройте маршрутизацию."
	case order.BranchID != nil:
		return "В выбранном филиале не найден руководитель или заместитель. Выберите исполнителя вручную или настройте маршрутизацию."
	case order.OfficeID != nil:
		return "В выбранном офисе не найден руководитель или заместитель. Выберите исполнителя вручную или настройте маршрутизацию."
	default:
		return "Для выбранной структуры не найден ответственный руководитель. Выберите исполнителя вручную или настройте маршрутизацию."
	}
}

func (s *OrderService) GetValidationConfigForOrderType(ctx context.Context, orderTypeID uint64) (map[string]interface{}, error) {
	code, err := s.orderTypeRepo.FindCodeByID(ctx, orderTypeID)
	if err != nil {
		return nil, err
	}
	if rules, ok := ValidationRegistry[code]; ok {
		m := make(map[string]interface{})
		for _, r := range rules {
			m[r.FieldName] = r.ErrorMessage
		}
		return m, nil
	}
	return map[string]interface{}{}, nil
}

func (s *OrderService) validateOrderRules(ctx context.Context, d dto.CreateOrderDTO) error {
	if d.OrderTypeID == nil {
		return nil
	}
	code, err := s.orderTypeRepo.FindCodeByID(ctx, *d.OrderTypeID)
	if err != nil {
		return nil
	}

	if rules, ok := OrderValidationRules[code]; ok {
		for _, field := range rules {
			if !s.checkFieldPresence(d, field) {
				return apperrors.NewBadRequestError(fmt.Sprintf("Поле %s обязательно.", field))
			}
		}
	}

	if code != "EQUIPMENT" {
		if !s.checkFieldPresence(d, "comment") {
			return apperrors.NewBadRequestError("Для данного типа заявки необходимо заполнить поле 'Комментарий'.")
		}
	}

	return nil
}

func (s *OrderService) checkFieldPresence(d dto.CreateOrderDTO, field string) bool {
	switch field {
	case "equipment_id":
		return d.EquipmentID != nil && *d.EquipmentID != 0
	case "equipment_type_id":
		return d.EquipmentTypeID != nil && *d.EquipmentTypeID != 0
	case "priority_id":
		return d.PriorityID != nil && *d.PriorityID != 0
	case "comment":
		return d.Comment != nil && strings.TrimSpace(*d.Comment) != ""
	default:
		return true
	}
}
