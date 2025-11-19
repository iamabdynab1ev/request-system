package onlinebank

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (p *Provider) fetchData(ctx context.Context, endpoint string) (json.RawMessage, error) {
	// Шаг 1: Получаем актуальный токен. Метод getToken() находится в auth.go.
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить токен аутентификации: %w", err)
	}

	// Шаг 2: Создаем GET-запрос с этим токеном.
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания GET-запроса: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// Шаг 3: Выполняем запрос.
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения GET-запроса для '%s': %w", endpoint, err)
	}
	defer resp.Body.Close()

	// Шаг 4: Проверяем статус ответа.
	if resp.StatusCode != http.StatusOK {
		// Здесь можно добавить логику чтения тела ошибки для более детальной диагностики
		return nil, fmt.Errorf("API банка для эндпоинта '%s' вернул статус: %s", endpoint, resp.Status)
	}

	// Шаг 5: ИЗМЕНЕНИЕ: Читаем "сырое" тело ответа с помощью io.ReadAll.
	return io.ReadAll(resp.Body)
}
