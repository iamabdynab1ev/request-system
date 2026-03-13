package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

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

func (c *UserController) BindADUsernamesByEmail(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	queryValues := ctx.Request().URL.Query()
	if sourceURL := strings.TrimSpace(ctx.QueryParam("source_url")); sourceURL != "" {
		parsedURL, err := url.Parse(sourceURL)
		if err != nil {
			return c.errorResponse(ctx, apperrors.NewBadRequestError("Некорректный source_url"))
		}
		queryValues = parsedURL.Query()
	}

	filter := utils.ParseFilterFromQuery(queryValues)
	users, err := c.userService.GetUsersForADBinding(reqCtx, filter)
	if err != nil {
		return c.errorResponse(ctx, err)
	}

	type failedBind struct {
		UserID   uint64      `json:"user_id"`
		Email    string      `json:"email"`
		Username string      `json:"username,omitempty"`
		Reason   string      `json:"reason"`
		Details  interface{} `json:"details,omitempty"`
	}

	type bindCandidate struct {
		UserID          uint64
		Email           string
		LocalPart       string
		CurrentUsername *string
	}

	result := struct {
		TotalUsers         int          `json:"total_users"`
		Updated            int          `json:"updated"`
		AlreadyMatched     int          `json:"already_matched"`
		NotFoundExactMatch []string     `json:"not_found_exact_match_emails"`
		InvalidEmails      []string     `json:"invalid_emails"`
		Failed             []failedBind `json:"failed"`
	}{
		TotalUsers: len(users),
	}

	candidates := make([]bindCandidate, 0, len(users))
	uniqueLocalParts := make(map[string]string, len(users))

	for _, user := range users {
		email := strings.TrimSpace(user.Email)
		if email == "" || !strings.Contains(email, "@") {
			result.InvalidEmails = append(result.InvalidEmails, email)
			c.logger.Warn("BindADUsernames: invalid email format", zap.Uint64("user_id", user.ID), zap.String("email", email))
			continue
		}

		localPart := strings.TrimSpace(strings.SplitN(email, "@", 2)[0])
		if localPart == "" {
			result.InvalidEmails = append(result.InvalidEmails, email)
			c.logger.Warn("BindADUsernames: empty local part in email", zap.Uint64("user_id", user.ID), zap.String("email", email))
			continue
		}

		candidates = append(candidates, bindCandidate{
			UserID:          user.ID,
			Email:           email,
			LocalPart:       localPart,
			CurrentUsername: user.Username,
		})

		localPartLower := strings.ToLower(localPart)
		if _, exists := uniqueLocalParts[localPartLower]; !exists {
			uniqueLocalParts[localPartLower] = localPart
		}
	}

	localParts := make([]string, 0, len(uniqueLocalParts))
	for _, localPart := range uniqueLocalParts {
		localParts = append(localParts, localPart)
	}

	exactMatches, err := c.adService.FindExactUsernames(localParts)
	if err != nil {
		return c.errorResponse(ctx, err)
	}

	updates := make(map[uint64]string, len(candidates))
	emailByUserID := make(map[uint64]string, len(candidates))

	for _, candidate := range candidates {
		matchedUsername, found := exactMatches[strings.ToLower(candidate.LocalPart)]
		if !found || strings.TrimSpace(matchedUsername) == "" {
			result.NotFoundExactMatch = append(result.NotFoundExactMatch, candidate.Email)
			c.logger.Warn("BindADUsernames: exact AD username not found", zap.Uint64("user_id", candidate.UserID), zap.String("email", candidate.Email), zap.String("search", candidate.LocalPart))
			continue
		}

		if candidate.CurrentUsername != nil && strings.EqualFold(strings.TrimSpace(*candidate.CurrentUsername), matchedUsername) {
			result.AlreadyMatched++
			continue
		}

		updates[candidate.UserID] = strings.TrimSpace(matchedUsername)
		emailByUserID[candidate.UserID] = candidate.Email
	}

	if len(updates) > 0 {
		failedUpdates := c.userService.SetUsernamesForADBinding(reqCtx, updates)
		for userID, username := range updates {
			if updateErr, failed := failedUpdates[userID]; failed {
				failure := failedBind{
					UserID: userID,
					Email:  emailByUserID[userID],
					Username: username,
					Reason: updateErr.Error(),
				}

				var httpErr *apperrors.HttpError
				if errors.As(updateErr, &httpErr) && httpErr.Details != nil {
					failure.Details = httpErr.Details
					c.logger.Warn(
						"BindADUsernames: failed to update user username",
						zap.Uint64("user_id", userID),
						zap.String("email", emailByUserID[userID]),
						zap.String("username", username),
						zap.Error(updateErr),
						zap.Any("details", httpErr.Details),
					)
				} else {
					c.logger.Warn("BindADUsernames: failed to update user username", zap.Uint64("user_id", userID), zap.String("email", emailByUserID[userID]), zap.String("username", username), zap.Error(updateErr))
				}

				result.Failed = append(result.Failed, failure)
				continue
			}

			result.Updated++
		}
	}

	return utils.SuccessResponse(ctx, result, "Массовая привязка username из AD завершена", http.StatusOK)
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
	contentType := ctx.Request().Header.Get("Content-Type")

	var formData dto.CreateUserDTO

	// ЛОГИКА ИСПРАВЛЕНИЯ
	if strings.HasPrefix(contentType, "application/json") {
		if err := ctx.Bind(&formData); err != nil {
			return c.errorResponse(ctx, apperrors.NewBadRequestError("Некорректный JSON в теле запроса"))
		}
	} else {
		dataString := ctx.FormValue("data")
		if dataString == "" {
			return c.errorResponse(ctx, apperrors.NewBadRequestError("Поле 'data' в form-data обязательно"))
		}
		if err := json.Unmarshal([]byte(dataString), &formData); err != nil {
			return c.errorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный JSON в поле 'data'", err, nil))
		}

		photoURL, err := c.handlePhotoUpload(ctx, constants.UploadContextProfilePhoto.String())
		if err != nil {
			return c.errorResponse(ctx, err)
		}
		formData.PhotoURL = photoURL
	}

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

	// Важно инициализировать мапу, иначе SmartUpdate не сработает
	var explicitFields map[string]interface{}

	contentType := ctx.Request().Header.Get("Content-Type")

	// 1. JSON
	if strings.HasPrefix(contentType, "application/json") {
		// Читаем тело запроса в байты
		bodyBytes, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			return c.errorResponse(ctx, apperrors.NewBadRequestError("Ошибка чтения тела запроса"))
		}

		// Восстанавливаем тело, чтобы Echo мог (если вдруг) прочитать его снова
		ctx.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Шаг А: Парсим в DTO (для валидации типов)
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return c.errorResponse(ctx, apperrors.NewBadRequestError("Некорректный JSON (типы данных)"))
		}

		// Шаг Б: Парсим в MAP (для SmartUpdate)
		if err := json.Unmarshal(bodyBytes, &explicitFields); err != nil {
			return c.errorResponse(ctx, apperrors.NewBadRequestError("Некорректный JSON (структура)"))
		}

	} else {
		// 2. MULTIPART FORM
		dataString := ctx.FormValue("data")

		if dataString != "" {
			// Парсим СТРУКТУРУ (для типов и валидации)
			if err := json.Unmarshal([]byte(dataString), &payload); err != nil {
				c.logger.Error("UpdateUser: JSON Unmarshal Error", zap.Error(err))
				return c.errorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный JSON в 'data'", err, nil))
			}

			// Парсим MAP (для SmartUpdate)
			if err := json.Unmarshal([]byte(dataString), &explicitFields); err != nil {
				return c.errorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Ошибка парсинга полей JSON", err, nil))
			}
		}

		photoURL, err := c.handlePhotoUpload(ctx, constants.UploadContextProfilePhoto.String())
		if err != nil {
			return c.errorResponse(ctx, err)
		}
		if photoURL != nil {
			payload.PhotoURL = photoURL
			if explicitFields == nil {
				explicitFields = make(map[string]interface{})
			}
			explicitFields["photo_url"] = *photoURL
		}
	}

	// Важно вернуть ID (json может его затереть, если там было поле id: 0 или null)
	payload.ID = idFromURL

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
