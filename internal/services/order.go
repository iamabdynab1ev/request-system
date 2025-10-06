package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"request-system/config"
	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type OrderServiceInterface interface {
	GetOrders(ctx context.Context, filter types.Filter) (*dto.OrderListResponseDTO, error)
	FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error)
	CreateOrder(ctx context.Context, createDTO dto.CreateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	DeleteOrder(ctx context.Context, orderID uint64) error
}

type OrderService struct {
	txManager    repositories.TxManagerInterface
	orderRepo    repositories.OrderRepositoryInterface
	userRepo     repositories.UserRepositoryInterface
	statusRepo   repositories.StatusRepositoryInterface
	priorityRepo repositories.PriorityRepositoryInterface
	attachRepo   repositories.AttachmentRepositoryInterface
	historyRepo  repositories.OrderHistoryRepositoryInterface
	fileStorage  filestorage.FileStorageInterface
	logger       *zap.Logger
}

func NewOrderService(
	txManager repositories.TxManagerInterface,
	orderRepo repositories.OrderRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface,
	attachRepo repositories.AttachmentRepositoryInterface,
	historyRepo repositories.OrderHistoryRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) OrderServiceInterface {
	return &OrderService{
		txManager:    txManager,
		orderRepo:    orderRepo,
		userRepo:     userRepo,
		statusRepo:   statusRepo,
		priorityRepo: priorityRepo,
		attachRepo:   attachRepo,
		historyRepo:  historyRepo,
		fileStorage:  fileStorage,
		logger:       logger,
	}
}

func (s *OrderService) GetOrders(ctx context.Context, filter types.Filter) (*dto.OrderListResponseDTO, error) {
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
	authContext := authz.Context{Actor: actor, Permissions: permissionsMap, Target: nil}
	if !authz.CanDo(authz.OrdersView, authContext) {
		return nil, apperrors.ErrForbidden
	}

	var securityFilter string
	var securityArgs []interface{}

	if !authContext.HasPermission(authz.ScopeAll) && !authContext.HasPermission(authz.ScopeAllView) {
		var conditions []string
		scopeRules := map[string]func(){
			authz.ScopeDepartment: func() {
				if actor.DepartmentID > 0 {
					conditions = append(conditions, "o.department_id = ?")
					securityArgs = append(securityArgs, actor.DepartmentID)
				}
			},
			// <<<--- НАЧАЛО: ИСПРАВЛЕНИЯ ТИПОВ В ПРОВЕРКАХ ---
			authz.ScopeBranch: func() {
				if actor.BranchID > 0 { // Сравниваем с 0, а не с nil
					conditions = append(conditions, "o.branch_id = ?")
					securityArgs = append(securityArgs, actor.BranchID)
				}
			},
			authz.ScopeOffice: func() {
				if actor.OfficeID != nil { // OfficeID, похоже, все же указатель, оставляем
					conditions = append(conditions, "o.office_id = ?")
					securityArgs = append(securityArgs, *actor.OfficeID)
				}
			},
			authz.ScopeOtdel: func() {
				if actor.OtdelID != nil { // OtdelID тоже, видимо, указатель, оставляем
					conditions = append(conditions, "o.otdel_id = ?")
					securityArgs = append(securityArgs, *actor.OtdelID)
				}
			},
			// <<<--- КОНЕЦ ИСПРАВЛЕНИЙ ---
			authz.ScopeOwn: func() {
				condition := `(o.user_id = ? OR o.executor_id = ? OR o.id IN (SELECT DISTINCT order_id FROM order_history WHERE user_id = ?))`
				conditions = append(conditions, condition)
				securityArgs = append(securityArgs, actor.ID, actor.ID, actor.ID)
			},
		}

		for scope, ruleFunc := range scopeRules {
			if authContext.HasPermission(scope) {
				ruleFunc()
			}
		}

		if len(conditions) == 0 {
			return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
		}
		securityFilter = "(" + strings.Join(conditions, " OR ") + ")"
	}

	orders, totalCount, err := s.orderRepo.GetOrders(ctx, filter, securityFilter, securityArgs)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
	}

	userIDsMap := make(map[uint64]struct{})
	orderIDs := make([]uint64, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
		userIDsMap[order.CreatorID] = struct{}{}
		if order.ExecutorID != nil {
			userIDsMap[*order.ExecutorID] = struct{}{}
		}
	}
	userIDs := make([]uint64, 0, len(userIDsMap))
	for id := range userIDsMap {
		userIDs = append(userIDs, id)
	}
	usersMap, err := s.userRepo.FindUsersByIDs(ctx, userIDs)
	if err != nil {
		s.logger.Error("GetOrders: не удалось получить пользователей по IDs", zap.Error(err))
		usersMap = make(map[uint64]entities.User)
	}
	attachmentsMap, err := s.attachRepo.FindAttachmentsByOrderIDs(ctx, orderIDs)
	if err != nil {
		s.logger.Error("GetOrders: не удалось получить вложения по IDs", zap.Error(err))
		attachmentsMap = make(map[uint64][]entities.Attachment)
	}
	dtos := make([]dto.OrderResponseDTO, 0, len(orders))
	for i := range orders {
		order := &orders[i]
		creator := usersMap[order.CreatorID]
		var executor entities.User
		if order.ExecutorID != nil {
			executor = usersMap[*order.ExecutorID]
		}
		attachments := attachmentsMap[order.ID]
		orderResponse := buildOrderResponse(order, &creator, &executor, attachments)
		dtos = append(dtos, *orderResponse)
	}
	return &dto.OrderListResponseDTO{List: dtos, TotalCount: totalCount}, nil
}

func (s *OrderService) FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersView, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	order := authContext.Target.(*entities.Order)
	creator, _ := s.userRepo.FindUserByID(ctx, order.CreatorID)
	var executor *entities.User
	if order.ExecutorID != nil && *order.ExecutorID > 0 {
		executor, _ = s.userRepo.FindUserByID(ctx, *order.ExecutorID)
	}
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, order.ID, 50, 0)

	return buildOrderResponse(order, creator, executor, attachments), nil
}

func (s *OrderService) CreateOrder(ctx context.Context, createDTO dto.CreateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, 0)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}
	if createDTO.ExecutorID != nil && !authz.CanDo(authz.OrdersCreateExecutorID, *authContext) {
		return nil, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав назначать исполнителя при создании.", nil, nil)
	}
	if createDTO.PriorityID != nil && !authz.CanDo(authz.OrdersCreatePriorityID, *authContext) {
		return nil, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав устанавливать приоритет при создании.", nil, nil)
	}

	var finalOrderID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		var executorIDToSet *uint64
		var executorToSet *entities.User

		if createDTO.ExecutorID != nil && *createDTO.ExecutorID > 0 {
			exc, err := s.userRepo.FindUserByIDInTx(ctx, tx, *createDTO.ExecutorID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusNotFound, "Указанный исполнитель не найден.", err, nil)
			}
			executorIDToSet = &exc.ID
			executorToSet = exc
		} else {
			head, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, createDTO.DepartmentID)
			if err != nil {
				if errors.Is(err, apperrors.ErrNotFound) {
					return apperrors.NewHttpError(http.StatusNotFound, "В целевом департаменте не назначен руководитель.", err, nil)
				}
				return err
			}
			executorIDToSet = &head.ID
			executorToSet = head
		}

		status, err := s.statusRepo.FindByCodeInTx(ctx, tx, "OPEN")
		if err != nil {
			return apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации: статус 'OPEN' не найден", err, nil)
		}

		var durationTime *time.Time
		if createDTO.Duration != nil && *createDTO.Duration != "" {
			parsedTime, err := time.Parse(time.RFC3339, *createDTO.Duration)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат даты 'duration', ожидается RFC3339.", err, nil)
			}
			durationTime = &parsedTime
		}

		orderEntity := &entities.Order{
			Name: createDTO.Name, Address: &createDTO.Address,
			DepartmentID: createDTO.DepartmentID, OtdelID: createDTO.OtdelID,
			BranchID: createDTO.BranchID, OfficeID: createDTO.OfficeID,
			EquipmentID: createDTO.EquipmentID, EquipmentTypeID: createDTO.EquipmentTypeID,
			// <<<--- ИСПРАВЛЕНИЕ 2: ПРИВЕДЕНИЕ ТИПА ---
			StatusID:   uint64(status.ID), // Приводим int к uint64
			PriorityID: createDTO.PriorityID,
			CreatorID:  authContext.Actor.ID, ExecutorID: executorIDToSet,
			Duration: durationTime,
		}
		orderID, err := s.orderRepo.Create(ctx, tx, orderEntity)
		if err != nil {
			return err
		}
		finalOrderID = orderID

		s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{OrderID: orderID, UserID: authContext.Actor.ID, EventType: "CREATE"}, nil)
		// <<<--- ИСПРАВЛЕНИЕ 3: ПРОВЕРКА И ПЕРЕДАЧА FIO ---
		if executorToSet != nil && executorToSet.Fio != "" {
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{OrderID: orderID, UserID: authContext.Actor.ID, EventType: "DELEGATION", NewValue: utils.StringToNullString(executorToSet.Fio)}, nil)
		}

		var attachmentIDForComment *uint64
		if file != nil {
			createdID, err := s.attachFileToOrderInTx(ctx, tx, file, orderID, authContext.Actor.ID)
			if err != nil {
				return err
			}
			attachmentIDForComment = &createdID
		}

		if createDTO.Comment != nil && *createDTO.Comment != "" {
			commentHistory := &entities.OrderHistory{
				OrderID: orderID, UserID: authContext.Actor.ID,
				EventType: "COMMENT", Comment: utils.StringPointerToNullString(createDTO.Comment),
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, commentHistory, attachmentIDForComment); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.FindOrderByID(ctx, finalOrderID)
}

func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	actor := authContext.Actor
	orderToUpdate := authContext.Target.(*entities.Order)
	originalOrder := *orderToUpdate

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		hasChanges := false

		if updateDTO.Name != nil && *updateDTO.Name != originalOrder.Name {
			if !authz.CanDo(authz.OrdersUpdateName, *authContext) {
				return apperrors.ErrForbidden
			}
			historyRecord := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "NAME_CHANGE",
				OldValue: utils.StringToNullString(originalOrder.Name),
				NewValue: utils.StringPointerToNullString(updateDTO.Name),
			}
			s.historyRepo.CreateInTx(ctx, tx, historyRecord, nil)
			orderToUpdate.Name = *updateDTO.Name
			hasChanges = true
		}

		var newExecutorID *uint64
		if updateDTO.DepartmentID != nil && *updateDTO.DepartmentID != originalOrder.DepartmentID {
			newHead, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, *updateDTO.DepartmentID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "В указанном департаменте нет руководителя.", err, nil)
			}
			newExecutorID = &newHead.ID
			orderToUpdate.DepartmentID = *updateDTO.DepartmentID
		} else if updateDTO.ExecutorID != nil {
			newExecutorID = updateDTO.ExecutorID
		}

		if newExecutorID != nil && !utils.AreUint64PointersEqual(newExecutorID, originalOrder.ExecutorID) {
			if !authz.CanDo(authz.OrdersUpdateExecutorID, *authContext) {
				return apperrors.ErrForbidden
			}
			newExec, err := s.userRepo.FindUserByIDInTx(ctx, tx, *newExecutorID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Указанный исполнитель не найден.", err, nil)
			}

			// <<<--- ИСПРАВЛЕНИЕ 4: ПРОВЕРКА И ПЕРЕДАЧА FIO ---
			if newExec.Fio != "" {
				s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", NewValue: utils.StringToNullString(newExec.Fio)}, nil)
			}
			orderToUpdate.ExecutorID = newExecutorID
			hasChanges = true
		}

		if updateDTO.StatusID != nil && *updateDTO.StatusID != originalOrder.StatusID {
			if !authz.CanDo(authz.OrdersUpdateStatusID, *authContext) {
				return apperrors.ErrForbidden
			}
			oldStatus, _ := s.statusRepo.FindStatusInTx(ctx, tx, originalOrder.StatusID)
			newStatus, err := s.statusRepo.FindStatusInTx(ctx, tx, *updateDTO.StatusID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Указанный статус не найден.", err, nil)
			}

			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "STATUS_CHANGE",
				OldValue: utils.StringToNullString(oldStatus.Name),
				NewValue: utils.StringToNullString(newStatus.Name),
			}, nil)
			orderToUpdate.StatusID = uint64(newStatus.ID) // Приводим int к uint64
			hasChanges = true
		}

		var attachmentIDForComment *uint64
		if file != nil {
			createdID, err := s.attachFileToOrderInTx(ctx, tx, file, orderID, actor.ID)
			if err != nil {
				return err
			}
			attachmentIDForComment = &createdID
		}
		if updateDTO.Comment != nil && *updateDTO.Comment != "" {
			historyComment := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "COMMENT",
				Comment: utils.StringPointerToNullString(updateDTO.Comment),
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, historyComment, attachmentIDForComment); err != nil {
				return err
			}
		}

		if hasChanges {
			if err = s.orderRepo.Update(ctx, tx, orderToUpdate); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.FindOrderByID(ctx, orderID)
}

func (s *OrderService) DeleteOrder(ctx context.Context, orderID uint64) error {
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.OrdersDelete, *authContext) {
		return apperrors.ErrForbidden
	}
	return s.orderRepo.DeleteOrder(ctx, orderID)
}

func (s *OrderService) buildAuthzContext(ctx context.Context, orderID uint64) (*authz.Context, error) {
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

	authContext := &authz.Context{Actor: actor, Permissions: permissionsMap}

	if orderID > 0 {
		targetOrder, err := s.orderRepo.FindByID(ctx, orderID)
		if err != nil {
			return nil, err
		}
		authContext.Target = targetOrder

		wasInHistory, err := s.historyRepo.IsUserParticipant(ctx, orderID, userID)
		if err != nil {
			s.logger.Warn("buildAuthzContext: ошибка при проверке участия в истории", zap.Error(err), zap.Uint64("orderID", orderID), zap.Uint64("userID", userID))
		}

		authContext.IsParticipant = targetOrder.CreatorID == userID ||
			(targetOrder.ExecutorID != nil && *targetOrder.ExecutorID == userID) ||
			wasInHistory
	}

	return authContext, nil
}

func (s *OrderService) attachFileToOrderInTx(ctx context.Context, tx pgx.Tx, file *multipart.FileHeader, orderID, userID uint64) (uint64, error) {
	src, err := file.Open()
	if err != nil {
		return 0, apperrors.NewHttpError(http.StatusInternalServerError, "Не удалось открыть файл", err, nil)
	}
	defer src.Close()

	const uploadContext = "order_document"
	if err = utils.ValidateFile(file, src, uploadContext); err != nil {
		return 0, apperrors.NewHttpError(http.StatusBadRequest, fmt.Sprintf("Файл не прошел валидацию: %s", err.Error()), err, nil)
	}

	rules, _ := config.UploadContexts[uploadContext]
	relativePath, err := s.fileStorage.Save(src, file.Filename, rules.PathPrefix)
	if err != nil {
		return 0, apperrors.NewHttpError(http.StatusInternalServerError, "Не удалось сохранить файл", err, nil)
	}

	fullFilePath := "/uploads/" + relativePath
	attach := &entities.Attachment{
		OrderID:  orderID,
		UserID:   userID,
		FileName: file.Filename,
		FilePath: fullFilePath,
		FileType: file.Header.Get("Content-Type"),
		FileSize: file.Size,
	}

	attachmentID, err := s.attachRepo.Create(ctx, tx, attach)
	if err != nil {
		s.fileStorage.Delete(fullFilePath)
		return 0, fmt.Errorf("не удалось создать запись о вложении в БД: %w", err)
	}

	attachHistory := &entities.OrderHistory{
		OrderID:   orderID,
		UserID:    userID,
		EventType: "ATTACHMENT_ADDED",
		NewValue:  utils.StringToNullString(file.Filename),
	}
	if err := s.historyRepo.CreateInTx(ctx, tx, attachHistory, &attachmentID); err != nil {
		return 0, err
	}

	return attachmentID, nil
}

func buildOrderResponse(
	order *entities.Order,
	creator *entities.User,
	executor *entities.User,
	attachments []entities.Attachment,
) *dto.OrderResponseDTO {
	creatorDTO := dto.ShortUserDTO{ID: order.CreatorID}
	// <<<--- ИСПРАВЛЕНИЕ 5: FIO - это string, а не *string ---
	if creator != nil && creator.ID != 0 {
		creatorDTO.Fio = creator.Fio
	}

	var executorDTO dto.ShortUserDTO
	if order.ExecutorID != nil {
		executorDTO.ID = *order.ExecutorID
		if executor != nil && executor.ID != 0 {
			executorDTO.Fio = executor.Fio
		}
	}

	var attachmentsDTO []dto.AttachmentResponseDTO
	for _, att := range attachments {
		attachmentsDTO = append(attachmentsDTO, dto.AttachmentResponseDTO{
			ID:       att.ID,
			FileName: att.FileName,
			FileSize: att.FileSize,
			FileType: att.FileType,
			URL:      att.FilePath,
		})
	}
	if attachmentsDTO == nil {
		attachmentsDTO = make([]dto.AttachmentResponseDTO, 0)
	}

	var durationStr *string
	if order.Duration != nil {
		formatted := order.Duration.Format(time.RFC3339)
		durationStr = &formatted
	}

	address := ""
	if order.Address != nil {
		address = *order.Address
	}

	return &dto.OrderResponseDTO{
		ID:              order.ID,
		Name:            order.Name,
		Address:         address,
		Creator:         creatorDTO,
		Executor:        executorDTO,
		DepartmentID:    order.DepartmentID,
		OtdelID:         order.OtdelID,
		BranchID:        order.BranchID,
		OfficeID:        order.OfficeID,
		EquipmentID:     order.EquipmentID,
		EquipmentTypeID: order.EquipmentTypeID,
		StatusID:        order.StatusID,
		PriorityID:      order.PriorityID,
		Attachments:     attachmentsDTO,
		Duration:        durationStr,
		CreatedAt:       order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       order.UpdatedAt.Format(time.RFC3339),
	}
}
