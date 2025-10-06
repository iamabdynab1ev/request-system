package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"request-system/config"
	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type UserController struct {
	userService services.UserServiceInterface
	fileStorage filestorage.FileStorageInterface
	logger      *zap.Logger
}

func NewUserController(
	userService services.UserServiceInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) *UserController {
	return &UserController{
		userService: userService,
		fileStorage: fileStorage,
		logger:      logger,
	}
}

func (c *UserController) errorResponse(ctx echo.Context, err error) error {
	return utils.ErrorResponse(ctx, err, c.logger)
}

func (c *UserController) GetUsers(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	res, totalCount, err := c.userService.GetUsers(reqCtx, filter)
	if err != nil {
		return c.errorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Пользователи успешно получены", http.StatusOK, totalCount)
}

func (c *UserController) FindUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return c.errorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID пользователя"))
	}
	res, err := c.userService.FindUser(reqCtx, id)
	if err != nil {
		return c.errorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Пользователь успешно найден", http.StatusOK)
}

func (c *UserController) CreateUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	dataString := ctx.FormValue("data")
	if dataString == "" {
		return c.errorResponse(ctx, apperrors.NewBadRequestError("Поле 'data' в form-data обязательно"))
	}
	var formData dto.CreateUserDTO
	if err := json.Unmarshal([]byte(dataString), &formData); err != nil {
		return c.errorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный JSON в поле 'data'", err, nil))
	}
	photoURL, err := c.handlePhotoUpload(ctx, constants.UploadContextProfilePhoto.String())
	if err != nil {
		return c.errorResponse(ctx, err)
	}
	formData.PhotoURL = photoURL
	if err := ctx.Validate(&formData); err != nil {
		return c.errorResponse(ctx, err)
	}
	res, err := c.userService.CreateUser(reqCtx, formData)
	if err != nil {
		return c.errorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Пользователь успешно создан", http.StatusCreated)
}

func (c *UserController) UpdateUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	idFromURL, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return c.errorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID пользователя"))
	}

	payload := dto.UpdateUserDTO{ID: idFromURL}

	// 2. Читаем JSON из текстового поля 'data'
	dataString := ctx.FormValue("data")
	if dataString != "" {
		if err := json.Unmarshal([]byte(dataString), &payload); err != nil {
			c.logger.Error("UpdateUser: ошибка десериализации 'data'", zap.Error(err))
			return c.errorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный JSON в поле 'data'", err, nil))
		}
	}

	photoURL, err := c.handlePhotoUpload(ctx, "profile_photo")
	if err != nil {
		return c.errorResponse(ctx, err)
	}

	if photoURL != nil {
		payload.PhotoURL = photoURL
	}

	if err = ctx.Validate(&payload); err != nil {
		return c.errorResponse(ctx, err)
	}

	res, err := c.userService.UpdateUser(reqCtx, payload)
	if err != nil {
		return c.errorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Пользователь успешно обновлен", http.StatusOK)
}

func (c *UserController) handlePhotoUpload(ctx echo.Context, uploadContext string) (*string, error) {
	file, err := ctx.FormFile("photoFile")
	if err != nil {
		if err == http.ErrMissingFile {
			return nil, nil
		}
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Ошибка при чтении файла", err, nil)
	}
	src, err := file.Open()
	if err != nil {
		return nil, apperrors.ErrInternalServer
	}
	defer src.Close()
	if err := utils.ValidateFile(file, src, uploadContext); err != nil {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Файл не прошел валидацию", err, nil)
	}
	rules, _ := config.UploadContexts[uploadContext]
	savedPath, err := c.fileStorage.Save(src, file.Filename, rules.PathPrefix)
	if err != nil {
		return nil, apperrors.ErrInternalServer
	}
	fileURL := "/uploads/" + savedPath
	return &fileURL, nil
}

func (c *UserController) DeleteUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return c.errorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID пользователя"))
	}

	if err := c.userService.DeleteUser(reqCtx, id); err != nil {
		return c.errorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Пользователь успешно удален", http.StatusOK)
}
