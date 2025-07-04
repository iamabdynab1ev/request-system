package controllers

import (
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// OrderController обрабатывает все HTTP-запросы, связанные с заявками.
type OrderController struct {
	orderService services.OrderServiceInterface
	logger       *zap.Logger
}

// NewOrderController является конструктором для OrderController.
// Он использует инъекцию зависимостей для получения сервиса заявок и логгера.
func NewOrderController(
	orderService services.OrderServiceInterface,
	logger *zap.Logger,
) *OrderController {
	return &OrderController{
		orderService: orderService,
		logger:       logger,
	}
}

// GetOrders получает список заявок с пагинацией.
func (c *OrderController) GetOrders(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	limit, offset, _ := utils.ParsePaginationParams(ctx.QueryParams())

	res, total, err := c.orderService.GetOrders(reqCtx, limit, offset)
	if err != nil {
		c.logger.Error("ошибка при получении списка заявок", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Список заявок успешно получен", http.StatusOK, total)
}

// FindOrder находит одну заявку по её ID.
func (c *OrderController) FindOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		// Улучшение: Логируем ошибку парсинга ID для полноты картины.
		c.logger.Error("некорректный ID заявки в URL", zap.Error(err), zap.String("id_param", ctx.Param("id")))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID заявки"))
	}

	res, err := c.orderService.FindOrder(reqCtx, id)
	if err != nil {
		c.logger.Error("ошибка при поиске заявки", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Заявка успешно найдена", http.StatusOK)
}

// CreateOrder создает новую заявку.
func (c *OrderController) CreateOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	var dto dto.CreateOrderDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос на создание заявки (bind)", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("ошибка валидации данных для новой заявки", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.orderService.CreateOrder(reqCtx, dto)
	if err != nil {
		c.logger.Error("ошибка при создании заявки", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Заявка успешно создана", http.StatusCreated)
}

// UpdateOrder обновляет существующую заявку.
func (c *OrderController) UpdateOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		// Улучшение: Добавлено логирование ошибки, как и в других методах.
		c.logger.Error("некорректный ID заявки в URL для обновления", zap.Error(err), zap.String("id_param", ctx.Param("id")))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID"))
	}

	var dto dto.UpdateOrderDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос на обновление заявки (bind)", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	// Замечание: Строка `dto.ID = int(id)` была удалена.
	// ID ресурса (заявки) должен приходить из URL, а тело запроса (DTO)
	// должно содержать только те поля, которые можно изменять.
	// Передача ID и в URL, и в теле может привести к путанице.
	// Сервисный метод `UpdateOrder` уже принимает `id` как отдельный аргумент.

	err = c.orderService.UpdateOrder(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("ошибка при обновлении заявки", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, nil, "Заявка успешно обновлена", http.StatusOK)
}

// DelegateOrder делегирует заявку другому исполнителю.
// НОВЫЙ МЕТОД: Реализует логику из вашего бизнес-требования.
func (c *OrderController) DelegateOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("некорректный ID заявки в URL для делегирования", zap.Error(err), zap.String("id_param", ctx.Param("id")))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID заявки"))
	}

	var dto dto.DelegateOrderDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос на делегирование заявки (bind)", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("ошибка валидации данных для делегирования", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	// Предполагается, что у вас есть такой метод в сервисе
	err = c.orderService.DelegateOrder(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("ошибка при делегировании заявки", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, nil, "Заявка успешно делегирована", http.StatusOK)
}

// SoftDeleteOrder выполняет "мягкое" удаление заявки.
func (c *OrderController) SoftDeleteOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		// Улучшение: Добавлено логирование ошибки.
		c.logger.Error("некорректный ID заявки в URL для удаления", zap.Error(err), zap.String("id_param", ctx.Param("id")))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID"))
	}
	err = c.orderService.SoftDeleteOrder(reqCtx, id)
	if err != nil {
		// Улучшение: Добавлен ID в лог ошибки для контекста.
		c.logger.Error("ошибка при мягком удалении заявки", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Заявка успешно удалена", http.StatusOK)
}
