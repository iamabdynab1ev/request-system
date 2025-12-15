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
	"request-system/pkg/validation"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type UserController struct {
	userService services.UserServiceInterface
	adService   services.ADServiceInterface
	fileStorage filestorage.FileStorageInterface
	logger      *zap.Logger
}

func NewUserController(
	userService services.UserServiceInterface,
	adService services.ADServiceInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) *UserController {
	return &UserController{
		userService: userService,
		adService:   adService,
		fileStorage: fileStorage,
		logger:      logger,
	}
}

func (c *UserController) SearchADUsers(ctx echo.Context) error {
	searchQuery := ctx.QueryParam("search")
	if len(searchQuery) < 3 {
		return c.errorResponse(ctx, apperrors.NewBadRequestError("Поисковый запрос должен содержать минимум 3 символа"))
	}

	users, err := c.adService.SearchUsers(searchQuery)
	if err != nil {
		return c.errorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, users, "Пользователи успешно найдены", http.StatusOK)
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
	var explicitFields map[string]interface{}

	dataString := ctx.FormValue("data")
	if dataString != "" {
		if err := json.Unmarshal([]byte(dataString), &payload); err != nil {
			c.logger.Error("UpdateUser: ошибка десериализации 'data' в DTO", zap.Error(err))
			return c.errorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный JSON в поле 'data'", err, nil))
		}
		if err := json.Unmarshal([]byte(dataString), &explicitFields); err != nil {
			c.logger.Error("UpdateUser: ошибка десериализации 'data' в map", zap.Error(err))
			return c.errorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный JSON (map)", err, nil))
		}
	}

	photoURL, err := c.handlePhotoUpload(ctx, constants.UploadContextProfilePhoto.String())
	if err != nil {
		return c.errorResponse(ctx, err)
	}
	if photoURL != nil {
		payload.PhotoURL = photoURL
	}

	if err = ctx.Validate(&payload); err != nil {
		return c.errorResponse(ctx, err)
	}

	res, err := c.userService.UpdateUser(reqCtx, payload, explicitFields)
	if err != nil {
		return c.errorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Пользователь успешно обновлен", http.StatusOK)
}

func (c *UserController) GetUserPermissions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return c.errorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID пользователя"))
	}
	res, err := c.userService.GetPermissionDetailsForUser(reqCtx, id)
	if err != nil {
		return c.errorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Права пользователя успешно получены", http.StatusOK)
}

func (c *UserController) UpdateUserPermissions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	userID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Warn("UpdateUserPermissions: некорректный ID", zap.Error(err))
		return c.errorResponse(ctx, apperrors.NewBadRequestError("Некорректный ID"))
	}
	var payload dto.UpdateUserPermissionsDTO
	if err := ctx.Bind(&payload); err != nil {
		return c.errorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных"))
	}
	if err := c.userService.UpdateUserPermissions(reqCtx, userID, payload); err != nil {
		return c.errorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, nil, "Права обновлены", http.StatusOK)
}

func (c *UserController) DeleteUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return c.errorResponse(ctx, apperrors.NewBadRequestError("Неверный ID"))
	}
	if err := c.userService.DeleteUser(reqCtx, id); err != nil {
		return c.errorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Пользователь удален", http.StatusOK)
}

func (c *UserController) handlePhotoUpload(ctx echo.Context, uploadContext string) (*string, error) {
	file, err := ctx.FormFile("photoFile")
	if err != nil {
		if err == http.ErrMissingFile {
			return nil, nil
		}
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Ошибка чтения файла", err, nil)
	}
	src, err := file.Open()
	if err != nil {
		return nil, apperrors.ErrInternalServer
	}
	defer src.Close()

	if err := validation.ValidateFile(file, src, uploadContext); err != nil {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Файл не прошел валидацию", err, nil)
	}

	rules, ok := config.UploadContexts[uploadContext]
	prefix := "uploads"
	if ok {
		prefix = rules.PathPrefix
	}

	savedPath, err := c.fileStorage.Save(src, file.Filename, prefix)
	if err != nil {
		return nil, apperrors.ErrInternalServer
	}
	fileURL := "/uploads/" + savedPath
	return &fileURL, nil
}
