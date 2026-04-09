package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type sampleUser struct {
	ID  uint64
	FIO string
}

type explainCase struct {
	name string
	sql  string
	args []interface{}
}

func main() {
	_ = godotenv.Load()

	dsn := firstNonEmpty(os.Getenv("DATABASE_URL"), os.Getenv("POSTGRES_DSN"))
	if strings.TrimSpace(dsn) == "" {
		log.Fatal("DATABASE_URL or POSTGRES_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping postgres: %v", err)
	}

	user, err := pickSampleUser(ctx, pool)
	if err != nil {
		log.Fatalf("pick sample user: %v", err)
	}

	orderID, err := pickSampleOrderID(ctx, pool)
	if err != nil {
		log.Fatalf("pick sample order id: %v", err)
	}

	searchTerm, err := pickSearchTerm(ctx, pool)
	if err != nil {
		log.Fatalf("pick search term: %v", err)
	}

	fmt.Printf("Sample user: #%d %s\n", user.ID, user.FIO)
	fmt.Printf("Sample order id: %d\n", orderID)
	fmt.Printf("Search term: %q\n\n", searchTerm)

	cases := []explainCase{
		{
			name: "telegram_all_page1",
			sql: `
SELECT o.id, o.name, o.address, o.status_id, o.user_id, o.executor_id, o.created_at
FROM orders o
WHERE o.deleted_at IS NULL
ORDER BY o.created_at DESC
LIMIT 10 OFFSET 0`,
		},
		{
			name: "telegram_my_tasks_page1",
			sql: `
SELECT o.id, o.name, o.address, o.status_id, o.user_id, o.executor_id, o.created_at
FROM orders o
WHERE o.deleted_at IS NULL
  AND o.user_id = $1
ORDER BY o.created_at DESC
LIMIT 10 OFFSET 0`,
			args: []interface{}{user.ID},
		},
		{
			name: "telegram_assigned_page1",
			sql: `
SELECT o.id, o.name, o.address, o.status_id, o.user_id, o.executor_id, o.created_at
FROM orders o
WHERE o.deleted_at IS NULL
  AND o.executor_id = $1
ORDER BY o.created_at DESC
LIMIT 10 OFFSET 0`,
			args: []interface{}{user.ID},
		},
		{
			name: "telegram_search_page1",
			sql: `
SELECT o.id, o.name, o.address, o.status_id, o.user_id, o.executor_id, o.created_at
FROM orders o
WHERE o.deleted_at IS NULL
  AND (o.name ILIKE $1 OR o.address ILIKE $1)
ORDER BY o.created_at DESC
LIMIT 20 OFFSET 0`,
			args: []interface{}{"%" + searchTerm + "%"},
		},
		{
			name: "telegram_exact_order_lookup",
			sql: `
SELECT o.id, o.name, o.address, o.status_id, o.user_id, o.executor_id, o.created_at
FROM orders o
WHERE o.deleted_at IS NULL
  AND o.id = $1`,
			args: []interface{}{orderID},
		},
		{
			name: "telegram_overdue_page1",
			sql: `
SELECT o.id, o.name, o.address, o.status_id, o.user_id, o.executor_id, o.created_at
FROM orders o
JOIN statuses s ON o.status_id = s.id
WHERE o.deleted_at IS NULL
  AND o.duration IS NOT NULL
  AND o.duration < NOW()
  AND s.code NOT IN ('CLOSED', 'COMPLETED', 'REJECTED')
ORDER BY o.created_at DESC
LIMIT 10 OFFSET 0`,
		},
	}

	for _, tc := range cases {
		if err := runExplain(ctx, pool, tc); err != nil {
			log.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func runExplain(ctx context.Context, pool *pgxpool.Pool, tc explainCase) error {
	fmt.Printf("=== %s ===\n", tc.name)

	query := "EXPLAIN (ANALYZE, BUFFERS, VERBOSE, FORMAT TEXT) " + strings.TrimSpace(tc.sql)
	rows, err := pool.Query(ctx, query, tc.args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return err
		}
		fmt.Println(line)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	fmt.Println()
	return nil
}

func pickSampleUser(ctx context.Context, pool *pgxpool.Pool) (*sampleUser, error) {
	query := `
SELECT u.id, u.fio
FROM users u
WHERE u.deleted_at IS NULL
  AND EXISTS (
    SELECT 1
    FROM orders o
    WHERE o.deleted_at IS NULL
      AND (o.user_id = u.id OR o.executor_id = u.id)
  )
ORDER BY u.id
LIMIT 1`

	var user sampleUser
	if err := pool.QueryRow(ctx, query).Scan(&user.ID, &user.FIO); err != nil {
		return nil, err
	}
	return &user, nil
}

func pickSampleOrderID(ctx context.Context, pool *pgxpool.Pool) (uint64, error) {
	query := `
SELECT o.id
FROM orders o
WHERE o.deleted_at IS NULL
ORDER BY o.created_at DESC
LIMIT 1`

	var id uint64
	if err := pool.QueryRow(ctx, query).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func pickSearchTerm(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	query := `
SELECT COALESCE(NULLIF(SUBSTRING(COALESCE(o.name, o.address) FROM 1 FOR 3), ''), 'ATM')
FROM orders o
WHERE o.deleted_at IS NULL
  AND COALESCE(o.name, o.address) IS NOT NULL
ORDER BY o.created_at DESC
LIMIT 1`

	var term string
	if err := pool.QueryRow(ctx, query).Scan(&term); err != nil {
		return "", err
	}

	term = strings.TrimSpace(term)
	if term == "" {
		return "", errors.New("empty search term")
	}
	return term, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
