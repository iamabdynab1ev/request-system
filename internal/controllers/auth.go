package controllers

import (
	"net/http"
	"time"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/service"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type AuthController struct {
	authService           services.AuthServiceInterface
	authPermissionService services.AuthPermissionServiceInterface
	jwtSvc                service.JWTService
	logger                *zap.Logger
}

func NewAuthController(
	authService services.AuthServiceInterface,
	authPermissionService services.AuthPermissionServiceInterface,
	jwtSvc service.JWTService,
	logger *zap.Logger,
) *AuthController {
	return &AuthController{
		authService:           authService,
		authPermissionService: authPermissionService,
		jwtSvc:                jwtSvc,
		logger:                logger,
	}
}

func (ctrl *AuthController) errorResponse(c echo.Context, err error) error {
	return utils.ErrorResponse(c, err, ctrl.logger)
}

func (ctrl *AuthController) Login(c echo.Context) error {
	var payload dto.LoginDTO

	if err := c.Bind(&payload); err != nil {
		ctrl.logger.Error("Login: ошибка привязки данных", zap.Error(err))
		return ctrl.errorResponse(c, apperrors.NewBadRequestError("Неверный формат данных для входа"))
	}

	if err := c.Validate(&payload); err != nil {
		ctrl.logger.Error("Login: ошибка валидации данных", zap.Error(err))
		return ctrl.errorResponse(c, err)
	}

	user, err := ctrl.authService.Login(c.Request().Context(), payload)
	if err != nil {
		ctrl.logger.Error("Login: ошибка авторизации", zap.String("login", payload.Login), zap.Error(err))
		return ctrl.errorResponse(c, err)
	}

	permissions, err := ctrl.authPermissionService.GetAllUserPermissions(c.Request().Context(), user.ID)
	if err != nil {
		ctrl.logger.Error("Login: не удалось получить привилегии пользователя", zap.Uint64("userID", user.ID), zap.Error(err))
		permissions = []string{}
	}

	return ctrl.generateTokensAndRespond(c, user.ID, permissions, "Авторизация прошла успешно", payload.RememberMe)
}

func (ctrl *AuthController) Logout(c echo.Context) error {
	cookie := &http.Cookie{
		Name:     "refreshToken",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}

	c.SetCookie(cookie)

	return utils.SuccessResponse(c, nil, "Вы успешно вышли из системы.", http.StatusOK)
}

func (ctrl *AuthController) RefreshToken(c echo.Context) error {
	cookie, err := c.Cookie("refreshToken")
	if err != nil {
		return utils.ErrorResponse(c, apperrors.ErrUnauthorized, ctrl.logger)
	}
	refreshTokenString := cookie.Value

	claims, err := ctrl.jwtSvc.ValidateToken(refreshTokenString)
	if err != nil {
		return utils.ErrorResponse(c, err, ctrl.logger)
	}

	if !claims.IsRefreshToken {
		return utils.ErrorResponse(
			c,
			apperrors.NewHttpError(
				http.StatusUnauthorized,
				"Для обновления должен использоваться Refresh токен",
				nil,
				nil,
			),
			ctrl.logger,
		)
	}

	permissions, err := ctrl.authPermissionService.GetAllUserPermissions(c.Request().Context(), claims.UserID)
	if err != nil {
		ctrl.logger.Error("Не удалось получить привилегии при обновлении токена", zap.Uint64("userID", claims.UserID), zap.Error(err))
		permissions = []string{}
	}

	return ctrl.generateTokensAndRespond(
		c,
		claims.UserID,
		permissions,
		"Токены успешно обновлены",
		true,
	)
}

func (ctrl *AuthController) Me(c echo.Context) error {
	userID, ok := c.Request().Context().Value(contextkeys.UserIDKey).(uint64)
	if !ok || userID == 0 {
		ctrl.logger.Error("Не удалось получить userID из контекста в защищенном маршруте")
		return utils.ErrorResponse(c, apperrors.ErrUnauthorized, ctrl.logger)
	}
	user, err := ctrl.authService.GetUserByID(c.Request().Context(), userID)
	if err != nil {
		ctrl.logger.Error("Ошибка получения пользователя по ID", zap.Uint64("userID", userID), zap.Error(err))
		return utils.ErrorResponse(c, err, ctrl.logger)
	}

	response := dto.UserProfileDTO{
		ID:           user.ID,
		Email:        user.Email,
		Phone:        user.PhoneNumber,
		FIO:          user.Fio,
		PhotoURL:     user.PhotoURL,
		PositionID:   user.PositionID,
		BranchID:     user.BranchID,
		DepartmentID: user.DepartmentID,
		OfficeID:     user.OfficeID,
		OtdelID:      user.OtdelID,
	}

	return utils.SuccessResponse(c, response, "Профиль пользователя успешно получен", http.StatusOK)
}

func (ctrl *AuthController) RequestPasswordReset(c echo.Context) error {
	var payload dto.ResetPasswordRequestDTO
	if err := c.Bind(&payload); err != nil {
		ctrl.logger.Error("Ошибка при привязке данных", zap.Error(err))
		return ctrl.errorResponse(c, err)
	}

	if err := c.Validate(&payload); err != nil {
		ctrl.logger.Error("Ошибка при валидации данных", zap.Any("payload", payload), zap.Error(err))
		return ctrl.errorResponse(c, err)
	}

	if err := ctrl.authService.RequestPasswordReset(c.Request().Context(), payload); err != nil {
		ctrl.logger.Error("Ошибка при запросе сброса пароля", zap.Any("payload", payload), zap.Error(err))
		return ctrl.errorResponse(c, err)
	}
	return utils.SuccessResponse(c, nil, "Если пользователь существует, инструкция будет отправлена.", http.StatusOK)
}

func (ctrl *AuthController) VerifyCode(c echo.Context) error {
	var payload dto.VerifyCodeDTO
	if err := c.Bind(&payload); err != nil {
		return ctrl.errorResponse(c, apperrors.ErrBadRequest)
	}
	if err := c.Validate(&payload); err != nil {
		return ctrl.errorResponse(c, err)
	}

	response, err := ctrl.authService.VerifyResetCode(c.Request().Context(), payload)
	if err != nil {
		return ctrl.errorResponse(c, err)
	}

	return utils.SuccessResponse(c, response, "Код подтвержден.", http.StatusOK)
}

func (ctrl *AuthController) ResetPassword(c echo.Context) error {
	var payload dto.ResetPasswordDTO
	if err := c.Bind(&payload); err != nil {
		return ctrl.errorResponse(c, apperrors.ErrBadRequest)
	}
	if err := c.Validate(&payload); err != nil {
		return ctrl.errorResponse(c, err)
	}

	if err := ctrl.authService.ResetPassword(c.Request().Context(), payload); err != nil {
		return ctrl.errorResponse(c, err)
	}
	return utils.SuccessResponse(c, nil, "Пароль успешно изменен.", http.StatusOK)
}

func (ctrl *AuthController) generateTokensAndRespond(c echo.Context, userID uint64, permissions []string, message string, rememberMe bool) error {
	accessTokenTTL := ctrl.jwtSvc.GetAccessTokenTTL()
	var refreshTokenTTL time.Duration

	if rememberMe {
		refreshTokenTTL = ctrl.jwtSvc.GetRefreshTokenTTL()
	} else {
		refreshTokenTTL = time.Hour * 8
	}

	accessToken, refreshToken, err := ctrl.jwtSvc.GenerateTokens(userID, 0, accessTokenTTL, refreshTokenTTL)
	if err != nil {
		ctrl.logger.Error("Не удалось сгенерировать токены", zap.Error(err), zap.Uint64("userID", userID))
		return ctrl.errorResponse(c, err)
	}

	cookie := new(http.Cookie)
	cookie.Name = "refreshToken"
	cookie.Value = refreshToken
	cookie.Path = "/"
	cookie.HttpOnly = true
	cookie.Secure = true
	cookie.SameSite = http.SameSiteNoneMode

	if rememberMe {
		cookie.Expires = time.Now().Add(refreshTokenTTL)
	}

	c.SetCookie(cookie)

	response := dto.AuthResponseDTO{
		AccessToken: accessToken,
		Permissions: permissions,
	}

	return utils.SuccessResponse(c, response, message, http.StatusOK)
}
