package controllers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/service"
	"request-system/pkg/utils"
)

type AuthController struct {
	authService *services.AuthService
	jwtService  service.JWTService
	logger      *zap.Logger
}

func NewAuthController(
	authService *services.AuthService,
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
	c.logger.Info("Login запрос получен")

	var payload dto.LoginDto
	if err := ctx.Bind(&payload); err != nil {
		c.logger.Warn("Login: Ошибка привязки тела запроса", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&payload); err != nil {
		c.logger.Warn("Login: Ошибка валидации данных запроса", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	user, err := c.authService.Login(ctx.Request().Context(), payload)
	if err != nil {
		c.logger.Error("Login: Ошибка авторизации пользователя", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	c.logger.Info("Login: Пользователь успешно авторизован", zap.Uint64("userID", user.ID))
	accessToken, refreshToken, err := c.jwtService.GenerateTokens(int(user.ID))
	if err != nil {
		c.logger.Error("Login: Не удалось сгенерировать токены", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("%w: %s", apperrors.ErrInvalidToken, "ошибка генерации токенов"))
	}
	c.logger.Info("Login: Токены успешно сгенерированы", zap.Uint64("userID", user.ID))
	accessTokenTTL := c.jwtService.GetAccessTokenTTL()
	refreshTokenTTL := c.jwtService.GetRefreshTokenTTL()

	c.logger.Info("Login: Получены TTL для токенов",
		zap.Duration("accessTokenTTL", accessTokenTTL),
		zap.Duration("refreshTokenTTL", refreshTokenTTL),
	)

	res := dto.AuthResponse{
		Token: dto.Token{
			AccessToken:           accessToken,
			RefreshToken:          refreshToken,
			AccessTokenExpiredIn:  int(accessTokenTTL.Seconds()),
			RefreshTokenExpiredIn: int(refreshTokenTTL.Seconds()),
		},
		User: *user,
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Авторизация прошла успешно",
		http.StatusOK,
	)
}

func (c *AuthController) Logout(ctx echo.Context) error {
	c.logger.Info("Logout запрос получен")
	return utils.SuccessResponse(ctx, nil, "Выход из системы успешно выполнен", http.StatusOK)
}

func (c *AuthController) Me(ctx echo.Context) error {
	userIDInterface := ctx.Get("UserID")
	if userIDInterface == nil {
		c.logger.Warn("Me: UserID не найден в контексте, возможно, middleware не отработал или токен невалиден")
		return utils.ErrorResponse(ctx, apperrors.ErrInvalidToken)
	}

	userID, ok := userIDInterface.(int)
	if !ok {
		c.logger.Error("Me: UserID в контексте имеет неверный тип", zap.Any("userIDValue", userIDInterface))
		return utils.ErrorResponse(ctx, apperrors.ErrInvalidToken)
	}

	c.logger.Info("Me запрос получен", zap.Int("userID", userID))

	return ctx.JSON(http.StatusOK, map[string]interface{}{"message": "Me success (заглушка)", "userID": userID})
}

func (h *AuthController) RefreshToken(ctx echo.Context) error {
	h.logger.Info("RefreshToken запрос получен")
	authHeader := ctx.Request().Header.Get("Authorization")
	parts := strings.Split(authHeader, " ")

	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		h.logger.Warn("RefreshToken: Неверный формат заголовка Authorization")
		return utils.ErrorResponse(ctx, apperrors.ErrInvalidAuthHeader)
	}
	refreshTokenString := parts[1]

	claims, err := h.jwtService.ValidateToken(refreshTokenString)
	if err != nil {
		h.logger.Warn("RefreshToken: Ошибка валидации refresh токена", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	if claims == nil {
		h.logger.Error("RefreshToken: ValidateToken вернул nil claims без ошибки (неожиданно)")
		return utils.ErrorResponse(ctx, apperrors.ErrInvalidToken)
	}

	if !claims.IsRefreshToken {
		h.logger.Warn("RefreshToken: Предоставленный токен не является refresh токеном", zap.Int("userID", claims.UserID))
		return utils.ErrorResponse(ctx, apperrors.ErrTokenIsNotRefresh)
	}
	h.logger.Info("RefreshToken: Refresh токен успешно валидирован", zap.Int("userID", claims.UserID))

	newAccessToken, newRefreshToken, errGenerate := h.jwtService.GenerateTokens(claims.UserID)
	if errGenerate != nil {
		h.logger.Error("RefreshToken: Ошибка генерации новой пары токенов", zap.Int("userID", claims.UserID), zap.Error(errGenerate))
		return utils.ErrorResponse(ctx, fmt.Errorf("%w: %s", apperrors.ErrInvalidToken, "ошибка генерации новой пары токенов"))
	}
	h.logger.Info("RefreshToken: Новая пара токенов успешно сгенерирована", zap.Int("userID", claims.UserID))

	newAccessTokenTTL := h.jwtService.GetAccessTokenTTL()
	newRefreshTokenTTL := h.jwtService.GetRefreshTokenTTL()

	tokenDto := &dto.Token{
		AccessToken:           newAccessToken,
		RefreshToken:          newRefreshToken,
		AccessTokenExpiredIn:  int(newAccessTokenTTL.Seconds()),
		RefreshTokenExpiredIn: int(newRefreshTokenTTL.Seconds()),
	}

	return utils.SuccessResponse(
		ctx,
		tokenDto,
		"Токены успешно обновлены",
		http.StatusOK,
	)
}

func (c *AuthController) Register(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "Регистрация прошла успешно (заглушка)")
}

func (c *AuthController) Verify(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "Подтверждение прошло успешно (заглушка)")
}

func (c *AuthController) Resend(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "Письмо повторно отправлено успешно (заглушка)")
}

func (c *AuthController) Reset(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "Сброс прошёл успешно (заглушка)")
}

func (c *AuthController) ChangePassword(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "Пароль успешно изменён (заглушка)")
}
