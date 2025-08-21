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
)

type OrderCommentController struct {
	orderCommentService services.OrderCommentServiceInterface
}

func NewOrderCommentController(
	orderCommentService services.OrderCommentServiceInterface,
) *OrderCommentController {
	return &OrderCommentController{
		orderCommentService: orderCommentService,
	}
}

func (c *OrderCommentController) GetOrderComments(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, total, err := c.orderCommentService.GetOrderComments(reqCtx, uint64(filter.Limit), uint64(filter.Offset))

	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Комментарии успешно получены", http.StatusOK, total)
}

func (c *OrderCommentController) FindOrderComment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("неверный ID комментария: %w", apperrors.ErrBadRequest))
	}

	res, err := c.orderCommentService.FindOrderComment(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Комментарий успешно найден", http.StatusOK)
}

// Этот метод создает новый комментарий. Все правильно.
func (c *OrderCommentController) CreateOrderComment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	var dto dto.CreateOrderCommentDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("ошибка данных в запросе: %w", apperrors.ErrBadRequest))
	}

	// Репозиторий сам возьмет ID пользователя из контекста
	newID, err := c.orderCommentService.CreateOrderComment(reqCtx, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, map[string]int{"id": newID}, "Комментарий успешно создан", http.StatusCreated)
}

// UpdateOrderComment остается без изменений
func (c *OrderCommentController) UpdateOrderComment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("неверный ID комментария в URL: %w", apperrors.ErrBadRequest))
	}

	var dto dto.UpdateOrderCommentDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("ошибка данных в запросе: %w", apperrors.ErrBadRequest))
	}

	err = c.orderCommentService.UpdateOrderComment(reqCtx, id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, nil, "Комментарий успешно обновлен", http.StatusOK)
}

// DeleteOrderComment остается без изменений
func (c *OrderCommentController) DeleteOrderComment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("неверный ID комментария: %w", apperrors.ErrBadRequest))
	}

	err = c.orderCommentService.DeleteOrderComment(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Комментарий успешно удален", http.StatusOK)
}
*/
