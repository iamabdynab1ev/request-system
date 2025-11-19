package onlinebank

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (p *Provider) getToken(ctx context.Context) (string, error) {
	p.tokenMutex.RLock()
	if p.token != "" && time.Now().Before(p.tokenExpiry.Add(-1*time.Minute)) {
		defer p.tokenMutex.RUnlock()
		return p.token, nil
	}
	p.tokenMutex.RUnlock()

	p.tokenMutex.Lock()
	defer p.tokenMutex.Unlock()

	// Повторная проверка внутри Lock на случай, если другой поток уже получил токен
	if p.token != "" && time.Now().Before(p.tokenExpiry.Add(-1*time.Minute)) {
		return p.token, nil
	}

	payloadString := fmt.Sprintf("grant_type=password&username=%s&password=%s",
		url.QueryEscape(p.username),
		url.QueryEscape(p.password),
	)
	payload := strings.NewReader(payloadString)

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/token", payload)
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса на аутентификацию: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "ARRAffinity=e2646848a81f31a47188609b8cd12df4ce3ccccf653e70836507f119d45ec2b8")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения запроса на аутентификацию: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// ИЗМЕНЕНИЕ: используем io.ReadAll
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		return "", fmt.Errorf("API аутентификации вернул статус: %s, тело ответа: %s", resp.Status, bodyString)
	}

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("ошибка парсинга ответа с токеном: %w", err)
	}

	if authResp.AccessToken == "" {
		return "", fmt.Errorf("API аутентификации не вернул access_token")
	}

	p.token = authResp.AccessToken
	p.tokenExpiry = time.Now().Add(time.Second * time.Duration(authResp.ExpiresIn))

	return p.token, nil
}
