package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"
)

const telegramWebhookSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

type config struct {
	URL          string
	Secret       string
	Requests     int
	Concurrency  int
	Mode         string
	InsecureTLS  bool
	BaseChatID   int64
	DistinctChat int
	ChatIDs      []int64
	ChatIDsRaw   string
	ChatIDsFile  string
	Text         string
	CallbackData string
	AgeSeconds   int
	Timeout      time.Duration
}

type result struct {
	statusCode int
	latency    time.Duration
	err        error
}

func main() {
	_ = godotenv.Load()

	cfg := parseFlags()

	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureTLS}, //nolint:gosec
		},
	}

	fmt.Printf("Telegram load test\n")
	fmt.Printf("URL: %s\n", cfg.URL)
	fmt.Printf("Mode: %s\n", cfg.Mode)
	fmt.Printf("Requests: %d\n", cfg.Requests)
	fmt.Printf("Concurrency: %d\n", cfg.Concurrency)
	fmt.Printf("Distinct chats: %d\n", cfg.DistinctChat)
	if len(cfg.ChatIDs) > 0 {
		fmt.Printf("Chat ids mode: explicit list (%d ids)\n", len(cfg.ChatIDs))
	} else {
		fmt.Printf("Base chat id: %d\n", cfg.BaseChatID)
	}
	fmt.Printf("Timeout: %s\n", cfg.Timeout)
	if strings.TrimSpace(cfg.Secret) != "" {
		fmt.Println("Webhook secret: enabled")
	} else {
		fmt.Println("Webhook secret: disabled")
	}
	fmt.Println()

	if cfg.DistinctChat == 1 {
		fmt.Println("Warning: one chat only. Telegram deduplicator/cooldown may hide real max throughput.")
		fmt.Println()
	}

	results := make(chan result, cfg.Requests)

	start := time.Now()
	var counter atomic.Int64
	var wg sync.WaitGroup

	for workerID := 0; workerID < cfg.Concurrency; workerID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				index := int(counter.Add(1))
				if index > cfg.Requests {
					return
				}

				chatID := pickChatID(cfg, index-1)
				updateID := 1_000_000 + index
				payload, err := buildPayload(cfg, updateID, chatID)
				if err != nil {
					results <- result{err: err}
					continue
				}

				req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, cfg.URL, bytes.NewReader(payload))
				if err != nil {
					results <- result{err: err}
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				if strings.TrimSpace(cfg.Secret) != "" {
					req.Header.Set(telegramWebhookSecretHeader, cfg.Secret)
				}

				reqStart := time.Now()
				resp, err := client.Do(req)
				latency := time.Since(reqStart)
				if err != nil {
					results <- result{latency: latency, err: err}
					continue
				}
				_ = resp.Body.Close()

				results <- result{
					statusCode: resp.StatusCode,
					latency:    latency,
				}
			}
		}()
	}

	wg.Wait()
	close(results)

	report(cfg, time.Since(start), results)
}

func parseFlags() config {
	defaultURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SERVER_BASE_URL")), "/") + "/api/webhooks/telegram"
	if strings.TrimSpace(os.Getenv("SERVER_BASE_URL")) == "" {
		defaultURL = "https://localhost:8091/api/webhooks/telegram"
	}

	cfg := config{}
	flag.StringVar(&cfg.URL, "url", defaultURL, "Telegram webhook URL")
	flag.StringVar(&cfg.Secret, "secret", "", "Telegram webhook secret token")
	flag.IntVar(&cfg.Requests, "requests", 200, "Total request count")
	flag.IntVar(&cfg.Concurrency, "concurrency", 20, "Concurrent workers")
	flag.StringVar(&cfg.Mode, "mode", "message", "Load mode: message, callback, mixed")
	flag.BoolVar(&cfg.InsecureTLS, "insecure", true, "Skip TLS verification")
	flag.Int64Var(&cfg.BaseChatID, "chat-id", 1000000000, "Base chat id for generated updates")
	flag.IntVar(&cfg.DistinctChat, "distinct-chats", 10, "How many distinct chat ids to rotate")
	flag.StringVar(&cfg.ChatIDsRaw, "chat-ids", "", "Comma-separated list of real chat ids")
	flag.StringVar(&cfg.ChatIDsFile, "chat-ids-file", "", "Path to file with one chat id per line")
	flag.StringVar(&cfg.Text, "text", "/help", "Telegram message text for message mode")
	flag.StringVar(&cfg.CallbackData, "callback", `{"action":"main_menu"}`, "Telegram callback data for callback mode")
	flag.IntVar(&cfg.AgeSeconds, "age-seconds", 0, "Artificial age for generated updates in seconds")
	flag.DurationVar(&cfg.Timeout, "timeout", 10*time.Second, "HTTP request timeout")
	flag.Parse()

	if strings.TrimSpace(cfg.Secret) == "" {
		cfg.Secret = strings.TrimSpace(os.Getenv("TELEGRAM_WEBHOOK_SECRET_TOKEN"))
	}

	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Mode != "message" && cfg.Mode != "callback" && cfg.Mode != "mixed" {
		log.Fatalf("unsupported mode: %s", cfg.Mode)
	}

	if cfg.Requests <= 0 {
		log.Fatal("requests must be > 0")
	}
	if cfg.Concurrency <= 0 {
		log.Fatal("concurrency must be > 0")
	}
	if cfg.DistinctChat <= 0 {
		log.Fatal("distinct-chats must be > 0")
	}

	chatIDs, err := loadChatIDs(cfg.ChatIDsRaw, cfg.ChatIDsFile)
	if err != nil {
		log.Fatalf("load chat ids: %v", err)
	}
	if len(chatIDs) > 0 {
		cfg.ChatIDs = chatIDs
		cfg.DistinctChat = len(chatIDs)
	}

	return cfg
}

func buildPayload(cfg config, updateID int, chatID int64) ([]byte, error) {
	now := time.Now().Add(-time.Duration(cfg.AgeSeconds) * time.Second).Unix()

	mode := cfg.Mode
	if mode == "mixed" {
		if updateID%2 == 0 {
			mode = "callback"
		} else {
			mode = "message"
		}
	}

	var payload any
	switch mode {
	case "callback":
		payload = map[string]any{
			"update_id": updateID,
			"callback_query": map[string]any{
				"id":   fmt.Sprintf("cb-%d", updateID),
				"from": map[string]any{"id": chatID},
				"message": map[string]any{
					"message_id": updateID,
					"date":       now,
					"text":       "loadtest callback host message",
					"chat":       map[string]any{"id": chatID},
					"from":       map[string]any{"id": chatID},
				},
				"data": cfg.CallbackData,
			},
		}
	default:
		payload = map[string]any{
			"update_id": updateID,
			"message": map[string]any{
				"message_id": updateID,
				"date":       now,
				"text":       cfg.Text,
				"chat":       map[string]any{"id": chatID},
				"from":       map[string]any{"id": chatID},
			},
		}
	}

	return json.Marshal(payload)
}

func report(cfg config, elapsed time.Duration, results <-chan result) {
	statusCounts := make(map[int]int)
	latencies := make([]float64, 0, cfg.Requests)
	errCount := 0
	successCount := 0

	for res := range results {
		if res.err != nil {
			errCount++
			continue
		}
		statusCounts[res.statusCode]++
		latencies = append(latencies, float64(res.latency.Milliseconds()))
		if res.statusCode >= 200 && res.statusCode < 300 {
			successCount++
		}
	}

	sort.Float64s(latencies)

	fmt.Println("Results")
	fmt.Printf("Elapsed: %s\n", elapsed)
	fmt.Printf("Throughput: %.2f req/s\n", float64(cfg.Requests)/elapsed.Seconds())
	fmt.Printf("HTTP 2xx: %d\n", successCount)
	fmt.Printf("Transport errors: %d\n", errCount)

	statusCodes := make([]int, 0, len(statusCounts))
	for code := range statusCounts {
		statusCodes = append(statusCodes, code)
	}
	sort.Ints(statusCodes)
	for _, code := range statusCodes {
		fmt.Printf("Status %d: %d\n", code, statusCounts[code])
	}

	if len(latencies) == 0 {
		fmt.Println("No successful HTTP responses to measure latency.")
		return
	}

	fmt.Printf("Latency p50: %.0f ms\n", percentile(latencies, 0.50))
	fmt.Printf("Latency p95: %.0f ms\n", percentile(latencies, 0.95))
	fmt.Printf("Latency p99: %.0f ms\n", percentile(latencies, 0.99))
	fmt.Printf("Latency max: %.0f ms\n", latencies[len(latencies)-1])
	fmt.Println()
	fmt.Println("Tip")
	fmt.Println("- Для ingress/очереди можно гонять random chats.")
	fmt.Println("- Для полной бизнес-нагрузки нужны реально привязанные chat_id, иначе часть Telegram-flow будет упираться в 'аккаунт не привязан'.")
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return values[0]
	}
	if p >= 1 {
		return values[len(values)-1]
	}

	index := int(float64(len(values)-1) * p)
	return values[index]
}

func loadChatIDs(raw string, filePath string) ([]int64, error) {
	var values []string

	if strings.TrimSpace(raw) != "" {
		values = append(values, strings.Split(raw, ",")...)
	}

	if strings.TrimSpace(filePath) != "" {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			values = append(values, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}

	chatIDs := make([]int64, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		chatID, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid chat id %q: %w", value, err)
		}
		chatIDs = append(chatIDs, chatID)
	}

	return chatIDs, nil
}

func pickChatID(cfg config, index int) int64 {
	if len(cfg.ChatIDs) > 0 {
		return cfg.ChatIDs[index%len(cfg.ChatIDs)]
	}

	return cfg.BaseChatID + int64(index%cfg.DistinctChat)
}
