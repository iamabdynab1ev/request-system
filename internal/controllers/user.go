package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"request-system/config"
	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	if logger == nil {
		logger = zap.New(zapcore.NewNopCore())
	}
	return &UserController{
		userService: userService,
		fileStorage: fileStorage,
		logger:      logger,
	}
}

func (c *UserController) GetUsers(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, totalCount, err := c.userService.GetUsers(reqCtx, filter)
	if err != nil {
		c.logger.Error("Ошибка при получении списка пользователей", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Пользователи успешно получены", http.StatusOK, totalCount)
}

func (c *UserController) FindUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Ошибка парсинга ID пользователя из URL", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid user ID format: %w", apperrors.ErrBadRequest))
	}

	res, err := c.userService.FindUser(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при поиске пользователя по ID", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *UserController) CreateUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	dataString := ctx.FormValue("data")
	if dataString == "" {
		return utils.ErrorResponse(ctx, fmt.Errorf("поле 'data' в form-data обязательно: %w", apperrors.ErrBadRequest))
	}

	var formData struct {
		Fio          string  `json:"fio"`
		Email        string  `json:"email"`
		PhoneNumber  string  `json:"phone_number"`
		Position     string  `json:"position"`
		Password     string  `json:"password"`
		StatusID     uint64  `json:"status_id"`
		RoleID       uint64  `json:"role_id"`
		BranchID     uint64  `json:"branch_id"`
		DepartmentID uint64  `json:"department_id"`
		OfficeID     *uint64 `json:"office_id"`
		OtdelID      *uint64 `json:"otdel_id"`
	}
	if err := json.Unmarshal([]byte(dataString), &formData); err != nil {
		c.logger.Error("Ошибка десериализации 'data'", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("некорректный JSON в поле 'data': %w", apperrors.ErrBadRequest))
	}

	finalDTO := dto.CreateUserDTO{
		Fio:          formData.Fio,
		Email:        formData.Email,
		PhoneNumber:  formData.PhoneNumber,
		Position:     formData.Position,
		Password:     formData.Password,
		StatusID:     formData.StatusID,
		RoleID:       formData.RoleID,
		BranchID:     formData.BranchID,
		DepartmentID: formData.DepartmentID,
		OfficeID:     formData.OfficeID,
		OtdelID:      formData.OtdelID,
	}

	file, err := ctx.FormFile("photoFile")
	if err != nil && err != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, fmt.Errorf("ошибка при чтении файла: %w", err))
	}

	if file != nil {
		src, err := file.Open()
		if err != nil {
			c.logger.Error("Не удалось открыть файл фото", zap.Error(err))
			return utils.ErrorResponse(ctx, apperrors.ErrInternalServer)
		}
		defer src.Close()

		const uploadContext = "profile_photo"

		if err := utils.ValidateFile(file, src, uploadContext); err != nil {
			return utils.ErrorResponse(ctx, fmt.Errorf("файл фото не прошел валидацию: %w", err))
		}

		rules, _ := config.UploadContexts[uploadContext]

		// Сохраняем файл, передавая префикс пути
		savedPath, err := c.fileStorage.Save(src, file.Filename, rules.PathPrefix)
		if err != nil {
			c.logger.Error("Ошибка сохранения фото профиля", zap.Error(err))
			return utils.ErrorResponse(ctx, apperrors.ErrInternalServer)
		}

		fileURL := "/uploads/" + savedPath
		finalDTO.PhotoURL = &fileURL
	}
	res, err := c.userService.CreateUser(reqCtx, finalDTO)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully created", http.StatusCreated)
}

func (c *UserController) UpdateUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	idFromURL, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Ошибка парсинга ID из URL для обновления", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid user ID: %w", apperrors.ErrBadRequest))
	}

	dataString := ctx.FormValue("data")
	finalDTO := dto.UpdateUserDTO{ID: idFromURL}

	if dataString != "" {
		var formData struct {
			Fio          string  `json:"fio"`
			Email        string  `json:"email"`
			PhoneNumber  string  `json:"phone_number"`
			Position     string  `json:"position"`
			Password     string  `json:"password"`
			StatusID     uint64  `json:"status_id"`
			RoleID       uint64  `json:"role_id"`
			BranchID     uint64  `json:"branch_id"`
			DepartmentID uint64  `json:"department_id"`
			OfficeID     *uint64 `json:"office_id"`
			OtdelID      *uint64 `json:"otdel_id"`
		}
		if err = json.Unmarshal([]byte(dataString), &formData); err != nil {
			c.logger.Error("Ошибка десериализации 'data' при обновлении", zap.Error(err))
			return utils.ErrorResponse(ctx, fmt.Errorf("некорректный JSON в 'data': %w", apperrors.ErrBadRequest))
		}

		finalDTO.Fio = formData.Fio
		finalDTO.Email = formData.Email
		finalDTO.PhoneNumber = formData.PhoneNumber
		finalDTO.Position = formData.Position
		finalDTO.Password = formData.Password
		finalDTO.StatusID = formData.StatusID
		finalDTO.RoleID = formData.RoleID
		finalDTO.BranchID = formData.BranchID
		finalDTO.DepartmentID = formData.DepartmentID
		finalDTO.OfficeID = formData.OfficeID
		finalDTO.OtdelID = formData.OtdelID
	}

	file, err := ctx.FormFile("photoFile")
	if err != nil && err != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, fmt.Errorf("ошибка при чтении файла: %w", err))
	}

	if file != nil {
		src, err := file.Open()
		if err != nil {
			c.logger.Error("Не удалось открыть файл фото при обновлении", zap.Error(err))
			return utils.ErrorResponse(ctx, apperrors.ErrInternalServer)
		}
		defer src.Close()

		const uploadContext = "profile_photo"

		if err := utils.ValidateFile(file, src, uploadContext); err != nil {
			return utils.ErrorResponse(ctx, fmt.Errorf("файл фото не прошел валидацию: %w", err))
		}

		rules, _ := config.UploadContexts[uploadContext]

		savedPath, err := c.fileStorage.Save(src, file.Filename, rules.PathPrefix)
		if err != nil {
			c.logger.Error("Ошибка сохранения фото профиля при обновлении", zap.Error(err))
			return utils.ErrorResponse(ctx, apperrors.ErrInternalServer)
		}

		fileURL := "/uploads/" + savedPath
		finalDTO.PhotoURL = &fileURL
	}

	if err = ctx.Validate(&finalDTO); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.userService.UpdateUser(reqCtx, finalDTO)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully updated", http.StatusOK)
}

func (c *UserController) DeleteUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Ошибка парсинга ID пользователя из URL для удаления", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid user ID format: %w", apperrors.ErrBadRequest))
	}

	err = c.userService.DeleteUser(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при мягком удалении пользователя в сервисе", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Successfully deleted", http.StatusOK)
}
