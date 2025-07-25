package controllers
/*
import (
	"fmt"
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type OrderDelegationController struct {
	orderDelegationService services.OrderDelegationServiceInterface
	logger                 *zap.Logger
}

func NewOrderDelegationController(
	orderDelegationService services.OrderDelegationServiceInterface,
	logger *zap.Logger,
) *OrderDelegationController {
	return &OrderDelegationController{
		orderDelegationService: orderDelegationService,
		logger:                 logger,
	}
}

func (c *OrderDelegationController) GetOrderDelegations(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, total, err := c.orderDelegationService.GetOrderDelegations(reqCtx, uint64(filter.Limit), uint64(filter.Offset))

	if err != nil {
		c.logger.Error("ошибка при получении списка делегирований", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Делегирования успешно получены", http.StatusOK, total)
}

func (c *OrderDelegationController) FindOrderDelegation(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Неверный формат ID делегирования", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("неверный ID делегирования"))
	}

	res, err := c.orderDelegationService.FindOrderDelegation(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при поиске делегирования", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Делегирование успешно найдено", http.StatusOK)
}

// Этот метод создает новое делегирование. Все правильно.
func (c *OrderDelegationController) CreateOrderDelegation(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	var dto dto.CreateOrderDelegationDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("ошибка данных в запросе: %w", apperrors.ErrBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("ошибка валидации: %w", err))
	}

	// Репозиторий сам возьмет ID пользователя (delegator_id) из контекста
	newID, err := c.orderDelegationService.CreateOrderDelegation(reqCtx, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, map[string]int{"id": newID}, "Делегирование успешно создано", http.StatusCreated)
}

// Этот метод удаляет делегирование. Все правильно.
func (c *OrderDelegationController) DeleteOrderDelegation(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Неверный формат ID для удаления", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("неверный формат ID"))
	}

	err = c.orderDelegationService.DeleteOrderDelegation(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Делегирование успешно удалено", http.StatusOK)
}
*/