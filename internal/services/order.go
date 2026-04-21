package services

import (
	"context"
	"mime/multipart"

	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/eventbus"
	"request-system/pkg/filestorage"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

type ValidationRule struct {
	FieldName    string
	ErrorMessage string
}

type orderFieldPermissionSpec struct {
	Permission string
	Label      string
}

var OrderValidationRules = map[string][]string{
	"EQUIPMENT": {"equipment_id", "equipment_type_id", "priority_id"},
}

var orderUpdateFieldPermissions = map[string]orderFieldPermissionSpec{
	"name":              {Permission: authz.OrdersUpdateName, Label: "название заявки"},
	"address":           {Permission: authz.OrdersUpdateAddress, Label: "адрес"},
	"department_id":     {Permission: authz.OrdersUpdateDepartmentID, Label: "департамент"},
	"otdel_id":          {Permission: authz.OrdersUpdateOtdelID, Label: "отдел"},
	"branch_id":         {Permission: authz.OrdersUpdateBranchID, Label: "филиал"},
	"office_id":         {Permission: authz.OrdersUpdateOfficeID, Label: "офис"},
	"equipment_id":      {Permission: authz.OrdersUpdateEquipmentID, Label: "оборудование"},
	"equipment_type_id": {Permission: authz.OrdersUpdateEquipmentTypeID, Label: "тип оборудования"},
	"executor_id":       {Permission: authz.OrdersUpdateExecutorID, Label: "исполнитель"},
	"status_id":         {Permission: authz.OrdersUpdateStatusID, Label: "статус"},
	"priority_id":       {Permission: authz.OrdersUpdatePriorityID, Label: "приоритет"},
	"duration":          {Permission: authz.OrdersUpdateDuration, Label: "срок"},
	"comment":           {Permission: authz.OrdersUpdateComment, Label: "комментарий"},
}

var orderCreateFieldPermissions = map[string]orderFieldPermissionSpec{
	"name":              {Permission: authz.OrdersCreateName, Label: "название заявки"},
	"address":           {Permission: authz.OrdersCreateAddress, Label: "адрес"},
	"department_id":     {Permission: authz.OrdersCreateDepartmentID, Label: "департамент"},
	"otdel_id":          {Permission: authz.OrdersCreateOtdelID, Label: "отдел"},
	"branch_id":         {Permission: authz.OrdersCreateBranchID, Label: "филиал"},
	"office_id":         {Permission: authz.OrdersCreateOfficeID, Label: "офис"},
	"equipment_id":      {Permission: authz.OrdersCreateEquipmentID, Label: "оборудование"},
	"equipment_type_id": {Permission: authz.OrdersCreateEquipmentTypeID, Label: "тип оборудования"},
	"executor_id":       {Permission: authz.OrdersCreateExecutorID, Label: "исполнитель"},
	"priority_id":       {Permission: authz.OrdersCreatePriorityID, Label: "приоритет"},
	"duration":          {Permission: authz.OrdersCreateDuration, Label: "срок"},
	"comment":           {Permission: authz.OrdersCreateComment, Label: "комментарий"},
}

var ValidationRegistry = map[string][]ValidationRule{
	"EQUIPMENT": {
		{FieldName: "equipment_id", ErrorMessage: "Пожалуйста, укажите оборудование."},
		{FieldName: "equipment_type_id", ErrorMessage: "Пожалуйста, выберите тип оборудования."},
		{FieldName: "priority_id", ErrorMessage: "Пожалуйста, укажите приоритет."},
	},
}

type OrderServiceInterface interface {
	CreateOrder(ctx context.Context, createDTO dto.CreateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	GetOrders(ctx context.Context, filter types.Filter, onlyCreated bool, onlyAssigned bool, onlyInvolved bool) (*dto.OrderListResponseDTO, error)
	FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error)
	UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader, explicitFields map[string]interface{}) (*dto.OrderResponseDTO, error)
	DeleteOrder(ctx context.Context, orderID uint64) error

	GetStatusByID(ctx context.Context, id uint64) (*entities.Status, error)
	GetPriorityByID(ctx context.Context, id uint64) (*entities.Priority, error)
	GetUserStats(ctx context.Context, userID uint64) (*types.UserOrderStats, error)
	GetValidationConfigForOrderType(ctx context.Context, orderTypeID uint64) (map[string]interface{}, error)
	FindOrderByIDForTelegram(ctx context.Context, userID uint64, orderID uint64) (*entities.Order, error)
}

type OrderService struct {
	txManager             repositories.TxManagerInterface
	orderRepo             repositories.OrderRepositoryInterface
	userRepo              repositories.UserRepositoryInterface
	statusRepo            repositories.StatusRepositoryInterface
	priorityRepo          repositories.PriorityRepositoryInterface
	attachRepo            repositories.AttachmentRepositoryInterface
	ruleEngine            RuleEngineServiceInterface
	historyRepo           repositories.OrderHistoryRepositoryInterface
	orderTypeRepo         repositories.OrderTypeRepositoryInterface
	fileStorage           filestorage.FileStorageInterface
	eventBus              *eventbus.Bus
	logger                *zap.Logger
	authPermissionService AuthPermissionServiceInterface
	notificationService   NotificationServiceInterface
	cacheRepo             repositories.CacheRepositoryInterface
}

func NewOrderService(
	txManager repositories.TxManagerInterface,
	orderRepo repositories.OrderRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface,
	attachRepo repositories.AttachmentRepositoryInterface,
	ruleEngine RuleEngineServiceInterface,
	historyRepo repositories.OrderHistoryRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	eventBus *eventbus.Bus,
	logger *zap.Logger,
	orderTypeRepo repositories.OrderTypeRepositoryInterface,
	authPermissionService AuthPermissionServiceInterface,
	notificationService NotificationServiceInterface,
	cacheRepo repositories.CacheRepositoryInterface,
) OrderServiceInterface {
	return &OrderService{
		txManager:             txManager,
		orderRepo:             orderRepo,
		userRepo:              userRepo,
		statusRepo:            statusRepo,
		priorityRepo:          priorityRepo,
		attachRepo:            attachRepo,
		ruleEngine:            ruleEngine,
		historyRepo:           historyRepo,
		fileStorage:           fileStorage,
		eventBus:              eventBus,
		logger:                logger,
		orderTypeRepo:         orderTypeRepo,
		authPermissionService: authPermissionService,
		notificationService:   notificationService,
		cacheRepo:             cacheRepo,
	}
}

func (s *OrderService) resolveActorFromContext(ctx context.Context, userID uint64) (*entities.User, error) {
	if actor, ok := ctx.Value(contextkeys.UserEntityKey).(*entities.User); ok && actor != nil && actor.ID == userID {
		return actor, nil
	}

	return s.userRepo.FindUserByID(ctx, userID)
}

func (s *OrderService) resolvePermissionsMap(ctx context.Context, userID uint64) (map[string]bool, error) {
	if permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx); err == nil && permissionsMap != nil {
		return permissionsMap, nil
	}

	permissions, err := s.authPermissionService.GetAllUserPermissions(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	permissionsMap := make(map[string]bool, len(permissions))
	for _, permission := range permissions {
		permissionsMap[permission] = true
	}

	return permissionsMap, nil
}
