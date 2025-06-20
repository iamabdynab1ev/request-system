// internal/controllers/auth_controller.go
package controllers

import (
	// Для reqCtx, если будете использовать
	// Псевдоним для стандартного пакета errors (для errors.Is)
	"fmt"
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors" // Псевдоним для вашего пакета ошибок
	"request-system/pkg/service"
	"request-system/pkg/utils"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type AuthController struct {
	authService *services.AuthService
	jwtService  service.JWTService // Этот jwtService будет передан извне
	logger      *zap.Logger
}

func NewAuthController(
	authService *services.AuthService,
	jwtService service.JWTService, // Принимаем jwtService
	logger *zap.Logger,
) *AuthController {
	return &AuthController{
		authService: authService,
		jwtService:  jwtService, // Используем переданный jwtService
		logger:      logger,
	}
}

func (c *AuthController) Login(ctx echo.Context) error {
	// reqCtx := ctx.Request().Context() // Можно использовать, если нужно передавать дальше
	c.logger.Info("Login запрос получен")

	var payload dto.LoginDto
	if err := ctx.Bind(&payload); err != nil {
		c.logger.Warn("Login: Ошибка привязки тела запроса", zap.Error(err))
		return utils.ErrorResponse(ctx, err) // Предполагается, что ErrorResponse корректно обрабатывает статус
	}

	if err := ctx.Validate(&payload); err != nil {
		c.logger.Warn("Login: Ошибка валидации данных запроса", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	user, err := c.authService.Login(ctx.Request().Context(), payload) // Используем контекст запроса Echo
	if err != nil {
		c.logger.Error("Login: Ошибка авторизации пользователя", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	c.logger.Info("Login: Пользователь успешно авторизован", zap.Int("userID", user.ID))

	accessToken, refreshToken, err := c.jwtService.GenerateTokens(user.ID)
	if err != nil {
		c.logger.Error("Login: Не удалось сгенерировать токены", zap.Error(err))
		// Здесь можно вернуть более специфичную ошибку из apperrors
		return utils.ErrorResponse(ctx, fmt.Errorf("%w: %s", apperrors.ErrInvalidToken, "ошибка генерации токенов"))
	}
	c.logger.Info("Login: Токены успешно сгенерированы", zap.Int("userID", user.ID))

	accessTokenTTL := c.jwtService.GetAccessTokenTTL()
	refreshTokenTTL := c.jwtService.GetRefreshTokenTTL()

	c.logger.Info("Login: Получены TTL для токенов",
		zap.Duration("accessTokenTTL", accessTokenTTL),
		zap.Duration("refreshTokenTTL", refreshTokenTTL),
	)

	// Предполагаем, что dto.User является DTO для ответа, а не entity.
	// Если user это *entities.User, его нужно смапить в dto.User.
	// var userDto dto.User // Замените на ваш User DTO
	// userDto = mapEntityToUserDto(user) // Пример функции маппинга

	res := dto.AuthResponse{
		Token: dto.Token{
			AccessToken:           accessToken,
			RefreshToken:          refreshToken,
			AccessTokenExpiredIn:  int(accessTokenTTL.Seconds()),  // TTL в секундах
			RefreshTokenExpiredIn: int(refreshTokenTTL.Seconds()), // TTL в секундах
		},
		User: *user, // Убедитесь, что user это DTO, а не entity, или смапьте его.
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Авторизация прошла успешно", // Сообщение на русском
		http.StatusOK,
	)
}

func (c *AuthController) Logout(ctx echo.Context) error {
	// Здесь может быть логика инвалидации токена (например, добавление в черный список)
	c.logger.Info("Logout запрос получен")
	return utils.SuccessResponse(ctx, nil, "Выход из системы успешно выполнен", http.StatusOK)
}

func (c *AuthController) Me(ctx echo.Context) error {
	// Этот эндпоинт должен быть защищен AuthMiddleware
	// UserID должен быть извлечен из контекста, который установил AuthMiddleware
	userIDInterface := ctx.Get("UserID") // Ключ, который использует AuthMiddleware
	if userIDInterface == nil {
		c.logger.Warn("Me: UserID не найден в контексте, возможно, middleware не отработал или токен невалиден")
		return utils.ErrorResponse(ctx, apperrors.ErrInvalidToken) // Или другая ошибка "unauthorized"
	}

	userID, ok := userIDInterface.(int)
	if !ok {
		c.logger.Error("Me: UserID в контексте имеет неверный тип", zap.Any("userIDValue", userIDInterface))
		return utils.ErrorResponse(ctx, apperrors.ErrInvalidToken)
	}

	c.logger.Info("Me запрос получен", zap.Int("userID", userID))
	// Здесь должна быть логика получения информации о пользователе по userID
	// user, err := c.authService.GetUserByID(ctx.Request().Context(), userID)
	// if err != nil { ... }
	// return utils.SuccessResponse(ctx, userDto, "Информация о пользователе", http.StatusOK)

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
	h.logger.Info("RefreshToken: Получен refresh токен", zap.String("tokenPreview", refreshTokenString[:10]+"...")) // Логгируем только часть токена

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

	h.logger.Info("RefreshToken: Получены TTL для новых токенов",
		zap.Duration("newAccessTokenTTL", newAccessTokenTTL),
		zap.Duration("newRefreshTokenTTL", newRefreshTokenTTL),
	)

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
