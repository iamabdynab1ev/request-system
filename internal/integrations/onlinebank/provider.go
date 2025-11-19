package onlinebank

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"request-system/internal/integrations"
	dto_internal "request-system/internal/integrations/dto"
)

// Provider - это "чистый фасад" для модуля OnlineBank.
type Provider struct {
	httpClient *http.Client
	baseURL    string
	username   string
	password   string
	logger     *zap.Logger // Добавляем логгер

	// Поля для кэширования токена
	token       string
	tokenExpiry time.Time
	tokenMutex  sync.RWMutex
}

// New - конструктор теперь принимает логгер.
func New(baseURL, username, password string, logger *zap.Logger) integrations.DataProvider {
	return &Provider{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		baseURL:    baseURL,
		username:   username,
		password:   password,
		logger:     logger.Named("onlinebank_provider"), // Даем логгеру имя для удобства
	}
}

// Name - реализуем метод интерфейса.
func (p *Provider) Name() string {
	return "onlinebank"
}

// processEntity - универсальная функция для получения, парсинга и маппинга любых сущностей.
// Ext - это тип DTO от внешнего API (например, onlinebank.BranchDTO)
// Int - это тип нашего внутреннего DTO (например, dto_internal.IntegrationBranchDTO)
func processEntity[Ext interface{ GetID() int }, Int any](
	p *Provider,
	ctx context.Context,
	endpoint string,
	mapper func(Ext) (Int, error),
) ([]Int, error) {
	// 1. Fetch
	rawData, err := p.fetchData(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения данных для эндпоинта %s: %w", endpoint, err)
	}

	// 2. Unmarshal
	var externalEntities []Ext
	if err := json.Unmarshal(rawData, &externalEntities); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON для эндпоинта %s: %w", endpoint, err)
	}
	p.logger.Debug("Успешно получено и распарсено",
		zap.String("endpoint", endpoint),
		zap.Int("count", len(externalEntities)),
	)

	// 3. Map
	internalEntities := make([]Int, 0, len(externalEntities))
	for _, entity := range externalEntities {
		internal, err := mapper(entity)
		if err != nil {
			// Используем структурированный логгер вместо fmt.Printf
			p.logger.Warn("Ошибка конвертации сущности, запись пропущена",
				zap.String("endpoint", endpoint),
				zap.Int("external_id", entity.GetID()),
				zap.Error(err),
			)
			continue
		}
		internalEntities = append(internalEntities, internal)
	}
	return internalEntities, nil
}

// GetBranches - тело метода теперь состоит из одной строки.
func (p *Provider) GetBranches(ctx context.Context) ([]dto_internal.IntegrationBranchDTO, error) {
	return processEntity(p, ctx, "/api/Reference/Branches", mapBranchToInternal)
}

// GetOffices - тело метода теперь состоит из одной строки.
func (p *Provider) GetOffices(ctx context.Context) ([]dto_internal.IntegrationOfficeDTO, error) {
	return processEntity(p, ctx, "/api/Reference/Offices", mapOfficeToInternal)
}
