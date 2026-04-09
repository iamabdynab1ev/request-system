package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"
)

type config struct {
	URL         string
	Method      string
	Token       string
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

	cfg := parseFlags()
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
	if strings.TrimSpace(cfg.Token) != "" {
		fmt.Println("Authorization: Bearer enabled")
	} else {
		fmt.Println("Authorization: disabled")
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

				results <- runOnce(client, cfg)
			}
		}()
	}

	wg.Wait()
	close(results)

	report(cfg, time.Since(start), results)
}

func parseFlags() config {
	defaultBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SERVER_BASE_URL")), "/")
	if defaultBaseURL == "" {
		defaultBaseURL = "https://localhost:8091"
	}

	cfg := config{}
	flag.StringVar(&cfg.URL, "url", defaultBaseURL+"/api/order?withPagination=true&page=1&limit=50", "Target API URL")
	flag.StringVar(&cfg.Method, "method", http.MethodGet, "HTTP method")
	flag.StringVar(&cfg.Token, "token", strings.TrimSpace(os.Getenv("LOADTEST_BEARER_TOKEN")), "Bearer token")
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

	return cfg
}

func runOnce(client *http.Client, cfg config) result {
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
	if strings.TrimSpace(cfg.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.Token))
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
