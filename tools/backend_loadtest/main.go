package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"request-system/pkg/service"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type config struct {
	URL         string
	Method      string
	Tokens      []string
	ContentType string
	Body        string
	Requests    int
	Concurrency int
	InsecureTLS bool
	Timeout     time.Duration
}

type result struct {
	statusCode int
	latency    time.Duration
	err        error
}

func main() {
	_ = godotenv.Load()

	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureTLS}, //nolint:gosec
		},
	}

	fmt.Printf("Backend load test\n")
	fmt.Printf("URL: %s\n", cfg.URL)
	fmt.Printf("Method: %s\n", cfg.Method)
	fmt.Printf("Requests: %d\n", cfg.Requests)
	fmt.Printf("Concurrency: %d\n", cfg.Concurrency)
	fmt.Printf("Timeout: %s\n", cfg.Timeout)
	switch len(cfg.Tokens) {
	case 0:
		fmt.Println("Authorization: disabled")
	case 1:
		fmt.Println("Authorization: Bearer enabled (single token)")
	default:
		fmt.Printf("Authorization: Bearer pool enabled (%d tokens)\n", len(cfg.Tokens))
	}
	fmt.Println()

	results := make(chan result, cfg.Requests)
	var counter atomic.Int64
	var wg sync.WaitGroup
	start := time.Now()

	for workerID := 0; workerID < cfg.Concurrency; workerID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				index := int(counter.Add(1))
				if index > cfg.Requests {
					return
				}

				results <- runOnce(client, cfg, index-1)
			}
		}()
	}

	wg.Wait()
	close(results)

	report(cfg, time.Since(start), results)
}

func parseFlags() (config, error) {
	defaultBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SERVER_BASE_URL")), "/")
	if defaultBaseURL == "" {
		defaultBaseURL = "https://localhost:8091"
	}

	cfg := config{}
	var singleToken string
	var tokensRaw string
	var tokensFile string
	var userIDsRaw string
	var userIDsFile string
	flag.StringVar(&cfg.URL, "url", defaultBaseURL+"/api/order?withPagination=true&page=1&limit=50", "Target API URL")
	flag.StringVar(&cfg.Method, "method", http.MethodGet, "HTTP method")
	flag.StringVar(&singleToken, "token", strings.TrimSpace(os.Getenv("LOADTEST_BEARER_TOKEN")), "Single Bearer token")
	flag.StringVar(&tokensRaw, "tokens", strings.TrimSpace(os.Getenv("LOADTEST_BEARER_TOKENS")), "Comma-separated Bearer tokens")
	flag.StringVar(&tokensFile, "tokens-file", strings.TrimSpace(os.Getenv("LOADTEST_BEARER_TOKENS_FILE")), "Path to file with one Bearer token per line")
	flag.StringVar(&userIDsRaw, "user-ids", "", "Comma-separated user IDs for local JWT generation")
	flag.StringVar(&userIDsFile, "user-ids-file", "", "Path to file with one user ID per line for local JWT generation")
	flag.StringVar(&cfg.ContentType, "content-type", "application/json", "Request Content-Type")
	flag.StringVar(&cfg.Body, "body", "", "Raw request body")
	flag.IntVar(&cfg.Requests, "requests", 200, "Total request count")
	flag.IntVar(&cfg.Concurrency, "concurrency", 20, "Concurrent workers")
	flag.BoolVar(&cfg.InsecureTLS, "insecure", true, "Skip TLS verification")
	flag.DurationVar(&cfg.Timeout, "timeout", 15*time.Second, "HTTP request timeout")
	flag.Parse()

	cfg.Method = strings.ToUpper(strings.TrimSpace(cfg.Method))
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	if cfg.Requests <= 0 {
		panic("requests must be > 0")
	}
	if cfg.Concurrency <= 0 {
		panic("concurrency must be > 0")
	}

	useGeneratedTokens := strings.TrimSpace(userIDsRaw) != "" || strings.TrimSpace(userIDsFile) != ""

	var tokens []string
	var err error
	if useGeneratedTokens {
		tokens, err = loadGeneratedTokens(userIDsRaw, userIDsFile)
		if err != nil {
			return config{}, err
		}
	} else {
		tokens, err = loadTokens(singleToken, tokensRaw, tokensFile)
		if err != nil {
			return config{}, err
		}
	}
	cfg.Tokens = tokens

	return cfg, nil
}

func runOnce(client *http.Client, cfg config, requestIndex int) result {
	var body io.Reader
	if cfg.Body != "" {
		body = bytes.NewBufferString(cfg.Body)
	}

	req, err := http.NewRequestWithContext(context.Background(), cfg.Method, cfg.URL, body)
	if err != nil {
		return result{err: err}
	}
	if cfg.ContentType != "" && cfg.Body != "" {
		req.Header.Set("Content-Type", cfg.ContentType)
	}
	if token := cfg.tokenFor(requestIndex); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	reqStart := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(reqStart)
	if err != nil {
		return result{latency: latency, err: err}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	return result{statusCode: resp.StatusCode, latency: latency}
}

func (cfg config) tokenFor(requestIndex int) string {
	if len(cfg.Tokens) == 0 {
		return ""
	}
	if len(cfg.Tokens) == 1 {
		return cfg.Tokens[0]
	}
	if requestIndex < 0 {
		requestIndex = 0
	}
	return cfg.Tokens[requestIndex%len(cfg.Tokens)]
}

func loadTokens(singleToken, tokensRaw, tokensFile string) ([]string, error) {
	tokens := make([]string, 0, 8)
	tokens = appendTokens(tokens, strings.Split(tokensRaw, ",")...)

	if strings.TrimSpace(tokensFile) != "" {
		fileTokens, err := loadTokensFromFile(tokensFile)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, fileTokens...)
	}

	if len(tokens) == 0 {
		tokens = appendTokens(tokens, singleToken)
	}

	return dedupeTokens(tokens), nil
}

func loadGeneratedTokens(userIDsRaw, userIDsFile string) ([]string, error) {
	userIDs := make([]uint64, 0, 8)

	parsedUserIDs, err := parseUserIDs(strings.Split(userIDsRaw, ",")...)
	if err != nil {
		return nil, err
	}
	userIDs = append(userIDs, parsedUserIDs...)

	if strings.TrimSpace(userIDsFile) != "" {
		fileUserIDs, err := loadUserIDsFromFile(userIDsFile)
		if err != nil {
			return nil, err
		}
		userIDs = append(userIDs, fileUserIDs...)
	}

	userIDs = dedupeUserIDs(userIDs)
	if len(userIDs) == 0 {
		return nil, nil
	}

	secretKey := strings.TrimSpace(os.Getenv("JWT_SECRET_KEY"))
	if secretKey == "" {
		return nil, errors.New("generate tokens: JWT_SECRET_KEY is not set")
	}

	jwtSvc := service.NewJWTService(secretKey, 24*time.Hour, 30*24*time.Hour, zap.NewNop())
	tokens := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		accessToken, _, err := jwtSvc.GenerateTokens(userID, 0, jwtSvc.GetAccessTokenTTL(), jwtSvc.GetRefreshTokenTTL())
		if err != nil {
			return nil, fmt.Errorf("generate tokens: user_id=%d: %w", userID, err)
		}
		tokens = append(tokens, accessToken)
	}

	return tokens, nil
}

func loadTokensFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load tokens file: %w", err)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	tokens := make([]string, 0, len(lines))
	for i, line := range lines {
		token := strings.TrimSpace(strings.TrimPrefix(line, "\uFEFF"))
		if token == "" || strings.HasPrefix(token, "#") {
			continue
		}
		if strings.Contains(token, " ") {
			return nil, fmt.Errorf("load tokens file: line %d contains spaces", i+1)
		}
		tokens = append(tokens, token)
	}

	if len(tokens) == 0 {
		return nil, errors.New("load tokens file: no valid tokens found")
	}

	return tokens, nil
}

func loadUserIDsFromFile(path string) ([]uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load user ids file: %w", err)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	userIDs, err := parseUserIDs(lines...)
	if err != nil {
		return nil, fmt.Errorf("load user ids file: %w", err)
	}
	if len(userIDs) == 0 {
		return nil, errors.New("load user ids file: no valid user IDs found")
	}

	return userIDs, nil
}

func appendTokens(tokens []string, rawTokens ...string) []string {
	for _, token := range rawTokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func dedupeTokens(tokens []string) []string {
	if len(tokens) <= 1 {
		return tokens
	}

	seen := make(map[string]struct{}, len(tokens))
	deduped := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		deduped = append(deduped, token)
	}
	return deduped
}

func parseUserIDs(values ...string) ([]uint64, error) {
	userIDs := make([]uint64, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.TrimPrefix(value, "\uFEFF"))
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		userID, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user id %q: %w", value, err)
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

func dedupeUserIDs(userIDs []uint64) []uint64 {
	if len(userIDs) <= 1 {
		return userIDs
	}

	seen := make(map[uint64]struct{}, len(userIDs))
	deduped := make([]uint64, 0, len(userIDs))
	for _, userID := range userIDs {
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		deduped = append(deduped, userID)
	}
	return deduped
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
