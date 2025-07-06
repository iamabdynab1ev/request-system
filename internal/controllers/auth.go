// Файл: internal/controllers/auth_controller.go
// ПОЛНАЯ ИСПРАВЛЕННАЯ ВЕРСИЯ

package controllers

import (
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/service"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type AuthController struct {
	authService services.AuthServiceInterface
	jwtService  service.JWTService
	logger      *zap.Logger
}

func NewAuthController(

	authService services.AuthServiceInterface,
	jwtService service.JWTService,
	logger *zap.Logger,
) *AuthController {
	return &AuthController{
		authService: authService,
		jwtService:  jwtService,
		logger:      logger,
	}
}

func (c *AuthController) Login(ctx echo.Context) error {
	var payload dto.LoginDTO
	if err := ctx.Bind(&payload); err != nil {
		c.logger.Warn("Login: не удалось прочитать тело запроса", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	if err := ctx.Validate(&payload); err != nil {
		c.logger.Warn("Login: невалидные данные", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	user, err := c.authService.Login(ctx.Request().Context(), payload)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return c.generateTokensAndRespond(ctx, user)
}

func (c *AuthController) SendCode(ctx echo.Context) error {
	var payload dto.SendCodeDTO
	if err := ctx.Bind(&payload); err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	if err := ctx.Validate(&payload); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	if payload.Email == "" && payload.Phone == "" {
		return utils.ErrorResponse(ctx, apperrors.ErrValidation)
	}

	if err := c.authService.SendVerificationCode(ctx.Request().Context(), payload); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, nil, "Если пользователь с указанными данными существует, код будет отправлен.", http.StatusOK)
}

func (c *AuthController) VerifyCode(ctx echo.Context) error {
	var payload dto.VerifyCodeDTO
	if err := ctx.Bind(&payload); err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	if err := ctx.Validate(&payload); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	user, err := c.authService.LoginWithCode(ctx.Request().Context(), payload)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return c.generateTokensAndRespond(ctx, user)
}

func (c *AuthController) generateTokensAndRespond(ctx echo.Context, user *entities.User) error {
	accessToken, refreshToken, err := c.jwtService.GenerateTokens(user.ID)
	if err != nil {
		c.logger.Error("Не удалось сгенерировать токены", zap.Error(err), zap.Int("userID", user.ID))
		return utils.ErrorResponse(ctx, apperrors.ErrInternalServer)
	}
	c.logger.Info("Токены успешно сгенерированы", zap.Int("userID", user.ID))

	userDto := dto.UserPublicDTO{
		ID:     user.ID,
		Email:  user.Email,
		Phone:  user.PhoneNumber,
		Fio:    user.FIO,
		RoleID: user.RoleID,
	}

	res := dto.AuthResponseDTO{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         userDto,
	}

	return utils.SuccessResponse(ctx, res, "Авторизация прошла успешно", http.StatusOK)
}
