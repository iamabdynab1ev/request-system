// Файл: internal/integrations/1c/router.go
package v1c

import (
	"request-system/internal/services"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"go.uber.org/zap"
)

// RegisterRoutes регистрирует все эндпоинты для интеграции с 1С.
func RegisterRoutes(
	apiGroup *echo.Group, // ОБРАТИТЕ ВНИМАНИЕ: Используем 'apiGroup', а не 'secureGroup'
	syncService services.SyncServiceInterface,
	apiKey string, // API-ключ для защиты эндпоинта
	logger *zap.Logger,
) {
	logger.Info("Инициализация роутера для вебхуков 1С...")

	// 1. Создаем middleware для проверки API-ключа
	apiKeyValidator := middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
		return key == apiKey, nil
	})

	// 2. Создаем контроллер
	controller := NewController(syncService, logger)

	// 3. Создаем отдельную группу для вебхуков 1С внутри /api/webhooks
	v1cGroup := apiGroup.Group("/webhooks/1c")

	// 4. Применяем защиту API-ключом ко всей группе
	v1cGroup.Use(apiKeyValidator)

	// 5. Регистрируем наш маршрут для приема справочников
	// Он будет доступен по POST /api/webhooks/1c/references
	v1cGroup.POST("/references", controller.HandleReferencesWebhook)
}
