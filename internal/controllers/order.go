// Файл: internal/controllers/order_controller.go
package controllers

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type OrderController struct {
	orderService services.OrderServiceInterface
	logger       *zap.Logger
}

func NewOrderController(
	orderService services.OrderServiceInterface,
	logger *zap.Logger,
) *OrderController {
	return &OrderController{
		orderService: orderService,
		logger:       logger,
	}
}

// GetOrders и FindOrder - без изменений.
func (c *OrderController) GetOrders(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	// <<<--- НАЧАЛО ИЗМЕНЕНИЙ ---
	c.logger.Debug("--- PARSING FILTER ---",
		zap.String("raw_query", ctx.Request().URL.RawQuery)) // Логируем исходную строку запроса

	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	// ВОТ САМАЯ ВАЖНАЯ СТРОКА ДЛЯ ДИАГНОСТИКИ:
	c.logger.Debug("Parsed filter object", zap.Any("filter_struct", filter))
	// <<<--- КОНЕЦ ИЗМЕНЕНИЙ ---

	orderListResponse, err := c.orderService.GetOrders(reqCtx, filter)
	if err != nil {
		c.logger.Error("GetOrders: ошибка при получении списка заявок", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusInternalServerError, "Не удалось получить список заявок", err, nil,
		), c.logger)
	}

	return utils.SuccessResponse(
		ctx,
		orderListResponse.List,
		"Список заявок успешно получен",
		http.StatusOK,
		orderListResponse.TotalCount,
	)
}

func (c *OrderController) FindOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("FindOrder: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный формат ID заявки", err, map[string]interface{}{"param": ctx.Param("id")}),
			c.logger,
		)
	}

	order, err := c.orderService.FindOrderByID(reqCtx, orderID)
	if err != nil {
		c.logger.Warn("FindOrder: ошибка при поиске заявки по ID", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, order, "Заявка успешно найдена", http.StatusOK)
}

// --- >>> НАЧАЛО ИСПРАВЛЕННОЙ ЛОГИКИ <<< ---

// CreateOrder - ПОЛНОСТЬЮ ПЕРЕПИСАН.
// Теперь он корректно работает с form-data, как вы и хотели.
func (c *OrderController) CreateOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	// Шаг 1: Получаем JSON-данные из поля 'data'
	dataString := ctx.FormValue("data")
	if dataString == "" {
		c.logger.Warn("CreateOrder: поле 'data' в form-data обязательно")
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Поле 'data' с JSON данными обязательно", nil, nil),
			c.logger,
		)
	}

	// Шаг 2: Получаем файл из поля 'file' (он может отсутствовать, это нормально)
	file, err := ctx.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		c.logger.Error("CreateOrder: ошибка при чтении файла", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Ошибка при чтении файла", err, nil),
			c.logger,
		)
	}

	// Шаг 3: ВЫЗЫВАЕМ СЕРВИС с правильными параметрами (строка и файл)
	// Важно, чтобы сигнатура в OrderServiceInterface была `CreateOrder(ctx, dataString, file)`
	res, err := c.orderService.CreateOrder(reqCtx, dataString, file)
	if err != nil {
		c.logger.Error("CreateOrder: сервис вернул ошибку", zap.Error(err))
		// Мы возвращаем ошибку из сервиса напрямую, так как сервис теперь
		// сам формирует правильный apperrors.HttpError.
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Заявка успешно создана",
		http.StatusCreated,
	)
}

// UpdateOrder - ПОЛНОСТЬЮ ПЕРЕПИСАН.
// Сохраняет старую логику работы с form-data.
func (c *OrderController) UpdateOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdateOrder: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный формат ID заявки", err, map[string]interface{}{"param": ctx.Param("id")}),
			c.logger,
		)
	}

	// Шаг 1: Получаем JSON из поля 'data' и парсим его в DTO
	dataString := ctx.FormValue("data")
	var updateDTO dto.UpdateOrderDTO // Начинаем с пустого DTO

	if dataString != "" {
		if err := json.Unmarshal([]byte(dataString), &updateDTO); err != nil {
			c.logger.Error("UpdateOrder: некорректный JSON в поле 'data'", zap.Error(err))
			return utils.ErrorResponse(ctx, apperrors.NewHttpError(
				http.StatusBadRequest, "некорректный JSON в поле 'data'", err, nil),
				c.logger,
			)
		}
	} else {
		// Эта часть нужна, если вы вдруг захотите отправить чистое JSON-тело без файла,
		// но лучше придерживаться одного формата - form-data.
		c.logger.Warn("UpdateOrder: поле 'data' не было предоставлено. Обновление не будет содержать данных.")
	}

	// Шаг 2: Валидируем данные DTO
	if err := ctx.Validate(&updateDTO); err != nil {
		c.logger.Error("UpdateOrder: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	// Шаг 3: Получаем файл (он может отсутствовать)
	var fileHeader *multipart.FileHeader // Объявляем переменную для файла
	file, err := ctx.FormFile("file")
	if err != nil {
		if err != http.ErrMissingFile { // Если ошибка - это не "файл отсутствует", то это реальная проблема
			c.logger.Error("UpdateOrder: ошибка при чтении файла", zap.Error(err))
			return utils.ErrorResponse(ctx, apperrors.NewHttpError(
				http.StatusBadRequest, "Ошибка чтения файла", err, nil),
				c.logger,
			)
		}
	} else {
		fileHeader = file // Если файл есть, присваиваем его переменной
	}

	// Шаг 4: Вызываем сервис
	updatedOrder, err := c.orderService.UpdateOrder(reqCtx, orderID, updateDTO, fileHeader)
	if err != nil {
		c.logger.Error("Ошибка при обновлении заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, updatedOrder, "Заявка успешно обновлена", http.StatusOK)
}

// DeleteOrder - без изменений.
func (c *OrderController) DeleteOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeleteOrder: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный формат ID заявки", err, map[string]interface{}{"param": ctx.Param("id")}),
			c.logger,
		)
	}

	if err := c.orderService.DeleteOrder(reqCtx, orderID); err != nil {
		c.logger.Error("DeleteOrder: ошибка при удалении заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusInternalServerError, "Не удалось удалить заявку", err, nil),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Заявка успешно удалена", http.StatusOK)
}
