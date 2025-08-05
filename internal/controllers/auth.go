package controllers

import (
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/services"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/service"
	"request-system/pkg/utils"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type AuthController struct {
	authService services.AuthServiceInterface
	jwtSvc      service.JWTService
	logger      *zap.Logger
}

func NewAuthController(authService services.AuthServiceInterface, jwtSvc service.JWTService, logger *zap.Logger) *AuthController {
	return &AuthController{
		authService: authService,
		jwtSvc:      jwtSvc,
		logger:      logger,
	}
}

func (ctrl *AuthController) Login(c echo.Context) error {
	var payload dto.LoginDTO
	if err := c.Bind(&payload); err != nil {
		return utils.ErrorResponse(c, apperrors.ErrBadRequest)
	}
	if err := c.Validate(&payload); err != nil {
		return utils.ErrorResponse(c, err)
	}

	user, err := ctrl.authService.Login(c.Request().Context(), payload)
	if err != nil {
		return utils.ErrorResponse(c, err)
	}

	return ctrl.generateTokensAndRespond(c, user, "Авторизация прошла успешно")
}

func (ctrl *AuthController) SendCode(c echo.Context) error {
	var payload dto.SendCodeDTO
	if err := c.Bind(&payload); err != nil {
		return utils.ErrorResponse(c, apperrors.ErrBadRequest)
	}
	if err := c.Validate(&payload); err != nil {
		return utils.ErrorResponse(c, err)
	}
	if payload.Email == "" && payload.Phone == "" {
		return utils.ErrorResponse(c, apperrors.ErrValidation)
	}

	if err := ctrl.authService.SendVerificationCode(c.Request().Context(), payload); err != nil {
		return utils.ErrorResponse(c, err)
	}

	return utils.SuccessResponse(c, nil, "Если пользователь с указанными данными существует, код будет отправлен.", http.StatusOK)
}

func (ctrl *AuthController) VerifyCode(c echo.Context) error {
	var payload dto.VerifyCodeDTO
	if err := c.Bind(&payload); err != nil {
		return utils.ErrorResponse(c, apperrors.ErrBadRequest)
	}
	if err := c.Validate(&payload); err != nil {
		return utils.ErrorResponse(c, err)
	}

	user, err := ctrl.authService.LoginWithCode(c.Request().Context(), payload)
	if err != nil {
		return utils.ErrorResponse(c, err)
	}

	return ctrl.generateTokensAndRespond(c, user, "Авторизация прошла успешно")
}

func (ctrl *AuthController) RefreshToken(c echo.Context) error {
	cookie, err := c.Cookie("refreshToken")
	if err != nil {
		ctrl.logger.Warn("Попытка обновления токена без cookie", zap.Error(err))
		return utils.ErrorResponse(c, apperrors.ErrUnauthorized)
	}
	refreshTokenString := cookie.Value
	if refreshTokenString == "" {
		return utils.ErrorResponse(c, apperrors.ErrUnauthorized)
	}

	userID, err := ctrl.jwtSvc.ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return utils.ErrorResponse(c, err)
	}

	user, err := ctrl.authService.GetUserByID(c.Request().Context(), userID)
	if err != nil {
		return utils.ErrorResponse(c, err)
	}

	return ctrl.generateTokensAndRespond(c, user, "Токены успешно обновлены")
}

func (ctrl *AuthController) CheckRecoveryOptions(c echo.Context) error {
	var payload dto.ForgotPasswordInitDTO
	if err := c.Bind(&payload); err != nil {
		return utils.ErrorResponse(c, apperrors.ErrBadRequest)
	}
	if err := c.Validate(&payload); err != nil {
		return utils.ErrorResponse(c, err)
	}

	options, err := ctrl.authService.CheckRecoveryOptions(c.Request().Context(), payload)
	if err != nil {
		return utils.ErrorResponse(c, err)
	}

	return utils.SuccessResponse(c, options, "", http.StatusOK)
}

func (ctrl *AuthController) SendRecoveryInstructions(c echo.Context) error {
	var payload dto.ForgotPasswordSendDTO
	if err := c.Bind(&payload); err != nil {
		return utils.ErrorResponse(c, apperrors.ErrBadRequest)
	}
	if err := c.Validate(&payload); err != nil {
		return utils.ErrorResponse(c, err)
	}

	if err := ctrl.authService.SendRecoveryInstructions(c.Request().Context(), payload); err != nil {
		return utils.ErrorResponse(c, err)
	}

	return utils.SuccessResponse(c, nil, "Если пользователь существует, инструкция будет отправлена выбранным способом.", http.StatusOK)
}

func (ctrl *AuthController) ResetPasswordWithEmail(c echo.Context) error {
	var payload dto.ResetPasswordEmailDTO
	if err := c.Bind(&payload); err != nil {
		return utils.ErrorResponse(c, apperrors.ErrBadRequest)
	}
	if err := c.Validate(&payload); err != nil {
		return utils.ErrorResponse(c, err)
	}

	if err := ctrl.authService.ResetPasswordWithEmail(c.Request().Context(), payload); err != nil {
		return utils.ErrorResponse(c, err)
	}

	return utils.SuccessResponse(c, nil, "Пароль успешно изменен.", http.StatusOK)
}

func (ctrl *AuthController) ResetPasswordWithPhone(c echo.Context) error {
	var payload dto.ResetPasswordPhoneDTO
	if err := c.Bind(&payload); err != nil {
		return utils.ErrorResponse(c, apperrors.ErrBadRequest)
	}
	if err := c.Validate(&payload); err != nil {
		return utils.ErrorResponse(c, err)
	}

	if err := ctrl.authService.ResetPasswordWithPhone(c.Request().Context(), payload); err != nil {
		return utils.ErrorResponse(c, err)
	}

	return utils.SuccessResponse(c, nil, "Пароль успешно изменен.", http.StatusOK)
}

func (ctrl *AuthController) generateTokensAndRespond(c echo.Context, user *entities.User, message string) error {
	accessToken, refreshToken, err := ctrl.jwtSvc.GenerateTokens(user.ID, user.RoleID)
	if err != nil {
		ctrl.logger.Error("Не удалось сгенерировать токены", zap.Error(err), zap.Uint64("userID", user.ID))
		return utils.ErrorResponse(c, apperrors.ErrInternalServer)
	}

	cookie := new(http.Cookie)
	cookie.Name = "refreshToken"
	cookie.Value = refreshToken
	cookie.Expires = time.Now().Add(ctrl.jwtSvc.GetRefreshTokenTTL())
	cookie.Path = "/"
	cookie.HttpOnly = true
	cookie.Secure = true
	cookie.SameSite = http.SameSiteNoneMode

	cookie.Partitioned = true

	c.SetCookie(cookie)

	response := dto.AuthResponseDTO{
		AccessToken: accessToken,
		User: dto.UserPublicDTO{
			ID:           user.ID,
			Email:        user.Email,
			Phone:        user.PhoneNumber,
			FIO:          user.Fio,
			RoleID:       user.RoleID,
			PhotoURL:     user.PhotoURL,
			Position:     user.Position,
			BranchID:     user.BranchID,
			DepartmentID: user.DepartmentID,
			OfficeID:     user.OfficeID,
			OtdelID:      user.OtdelID,
		},
	}

	return utils.SuccessResponse(c, response, message, http.StatusOK)
}
func (ctrl *AuthController) Me(c echo.Context) error {
	userID, ok := c.Request().Context().Value(contextkeys.UserIDKey).(uint64)
	if !ok || userID == 0 {
		ctrl.logger.Error("Не удалось получить userID из контекста в защищенном маршруте")
		return utils.ErrorResponse(c, apperrors.ErrUnauthorized)
	}
	permissions, ok := c.Request().Context().Value(contextkeys.UserPermissionsKey).([]string)
	if !ok {
		ctrl.logger.Error("Привилегии не найдены в контексте для аутентифицированного пользователя", zap.Uint64("userID", userID))
		return utils.ErrorResponse(c, apperrors.ErrInternalServer)
	}

	ctrl.logger.Info("Запрос на получение данных и привилегий пользователя", zap.Uint64("userID", userID))

	user, err := ctrl.authService.GetUserByID(c.Request().Context(), userID)
	if err != nil {
		return utils.ErrorResponse(c, err)
	}

	response := dto.UserProfileDTO{
		ID:           user.ID,
		Email:        user.Email,
		Phone:        user.PhoneNumber,
		FIO:          user.Fio,
		RoleID:       user.RoleID,
		Permissions:  permissions,
		PhotoURL:     user.PhotoURL,
		Position:     user.Position,
		BranchID:     user.BranchID,
		DepartmentID: user.DepartmentID,
		OfficeID:     user.OfficeID,
		OtdelID:      user.OtdelID,
	}

	return utils.SuccessResponse(c, response, "Профиль пользователя успешно получен", http.StatusOK)
}
