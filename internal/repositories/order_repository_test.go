package repositories

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	apperrors "request-system/pkg/errors"
	"testing"
	"time"

	"request-system/internal/dto"
	"request-system/pkg/contextkeys"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testPool *pgxpool.Pool

// TestMain настраивает и разрывает соединение с тестовой БД, применяет схему и запускает тесты.
func TestMain(m *testing.M) {
	testDbUrl := "postgres://postgres:postgres@localhost:5432/request-system-test?sslmode=disable"
	var err error

	testPool, err = pgxpool.New(context.Background(), testDbUrl)
	if err != nil {
		log.Fatalf("Не удалось подключиться к тестовой БД: %v", err)
	}
	defer testPool.Close()

	applySchema(testPool)

	code := m.Run()
	os.Exit(code)
}

// applySchema читает и выполняет DDL-скрипт для создания таблиц в тестовой БД.
func applySchema(pool *pgxpool.Pool) {
	path, _ := filepath.Abs("../../testdata/schema.sql")
	schema, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("Не удалось прочитать schema.sql: %v", err)
	}
	_, err = pool.Exec(context.Background(), string(schema))
	if err != nil {
		log.Fatalf("Не удалось применить схему БД: %v", err)
	}
}

// cleanupTables очищает таблицы для обеспечения изоляции тестов.
func cleanupTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `TRUNCATE TABLE order_comments, order_delegations, orders, users, statuses, proreties RESTART IDENTITY CASCADE;`)
	require.NoError(t, err, "Не удалось очистить таблицы")
}

// seedData создает начальные данные (пользователи, статусы), необходимые для тестов.
func seedData(t *testing.T, pool *pgxpool.Pool) (creatorID int, executorID int, statusID int, proretyID int) {
	t.Helper()
	err := pool.QueryRow(context.Background(), `INSERT INTO users (fio) VALUES ('Тестовый Создатель') RETURNING id`).Scan(&creatorID)
	require.NoError(t, err)

	err = pool.QueryRow(context.Background(), `INSERT INTO users (fio) VALUES ('Тестовый Исполнитель') RETURNING id`).Scan(&executorID)
	require.NoError(t, err)

	err = pool.QueryRow(context.Background(), `INSERT INTO statuses (name) VALUES ('Новая') RETURNING id`).Scan(&statusID)
	require.NoError(t, err)

	err = pool.QueryRow(context.Background(), `INSERT INTO proreties (name) VALUES ('Обычный') RETURNING id`).Scan(&proretyID)
	require.NoError(t, err)

	return
}

func TestOrderRepository_Integration_CreateOrder(t *testing.T) {
	require.NotNil(t, testPool, "testPool не инициализирован")
	cleanupTables(t, testPool)
	creatorID, executorID, statusID, proretyID := seedData(t, testPool)
	repo := NewOrderRepository(testPool)

	createDto := dto.CreateOrderDTO{
		Name:         "Интеграционная тестовая заявка",
		ProretyID:    proretyID,
		StatusID:     statusID,
		ExecutorID:   executorID,
		Massage:      "Начальный комментарий к заявке",
		DepartmentID: 1,
	}

	newID, err := repo.CreateOrder(context.Background(), creatorID, createDto)
	require.NoError(t, err)
	require.True(t, newID > 0)

	var (
		orderName       string
		commentCount    int
		delegationCount int
	)
	err = testPool.QueryRow(context.Background(), "SELECT name FROM orders WHERE id = $1", newID).Scan(&orderName)
	require.NoError(t, err)
	assert.Equal(t, "Интеграционная тестовая заявка", orderName)

	err = testPool.QueryRow(context.Background(), "SELECT COUNT(*) FROM order_comments WHERE order_id = $1", newID).Scan(&commentCount)
	require.NoError(t, err)
	assert.Equal(t, 1, commentCount, "Должен быть создан один комментарий")

	err = testPool.QueryRow(context.Background(), "SELECT COUNT(*) FROM order_delegations WHERE order_id = $1", newID).Scan(&delegationCount)
	require.NoError(t, err)
	assert.Equal(t, 1, delegationCount, "Должна быть создана одна запись о делегировании")
}

func TestOrderRepository_Integration_FindOrder(t *testing.T) {
	cleanupTables(t, testPool)
	creatorID, executorID, statusID, proretyID := seedData(t, testPool)
	repo := NewOrderRepository(testPool)

	createDto := dto.CreateOrderDTO{Name: "Order to find", StatusID: statusID, ProretyID: proretyID, ExecutorID: executorID, Massage: "..."}
	newID, err := repo.CreateOrder(context.Background(), creatorID, createDto)
	require.NoError(t, err, "Подготовка теста FindOrder: создание заявки не должно вызывать ошибок")

	t.Run("success find", func(t *testing.T) {
		foundOrder, err := repo.FindOrder(context.Background(), uint64(newID))
		require.NoError(t, err)
		require.NotNil(t, foundOrder)
		assert.Equal(t, "Order to find", foundOrder.Name)
		assert.Equal(t, newID, foundOrder.ID)
		require.NotNil(t, foundOrder.Executor)
		assert.Equal(t, executorID, foundOrder.Executor.ID)
	})

	t.Run("not found", func(t *testing.T) {
		order, err := repo.FindOrder(context.Background(), 99999)
		require.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
		assert.Nil(t, order)
	})
}

func TestOrderRepository_Integration_UpdateOrder(t *testing.T) {
	cleanupTables(t, testPool)
	creatorID, executorID, statusID, proretyID := seedData(t, testPool)
	repo := NewOrderRepository(testPool)

	createDto := dto.CreateOrderDTO{Name: "Initial Name", StatusID: statusID, ExecutorID: executorID, ProretyID: proretyID, Massage: "..."}
	newID, err := repo.CreateOrder(context.Background(), creatorID, createDto)
	require.NoError(t, err, "Подготовка теста UpdateOrder: создание заявки не должно вызывать ошибок")

	var newStatusID, newExecutorID int
	err = testPool.QueryRow(context.Background(), "INSERT INTO statuses(name) VALUES ('В работе') RETURNING id").Scan(&newStatusID)
	require.NoError(t, err)
	err = testPool.QueryRow(context.Background(), "INSERT INTO users(fio) VALUES ('Новый исполнитель') RETURNING id").Scan(&newExecutorID)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), contextkeys.UserIDKey, creatorID)
	updateDto := dto.UpdateOrderDTO{Name: "Updated Name!", StatusID: newStatusID, ExecutorID: newExecutorID}

	err = repo.UpdateOrder(ctx, uint64(newID), updateDto)
	require.NoError(t, err)

	updatedOrder, _ := repo.FindOrder(context.Background(), uint64(newID))
	require.NotNil(t, updatedOrder)
	assert.Equal(t, "Updated Name!", updatedOrder.Name)
	assert.Equal(t, newStatusID, updatedOrder.Status.ID)
	require.NotNil(t, updatedOrder.Executor)
	assert.Equal(t, newExecutorID, updatedOrder.Executor.ID)
}

func TestOrderRepository_Integration_SoftDeleteOrder(t *testing.T) {
	cleanupTables(t, testPool)
	creatorID, executorID, statusID, proretyID := seedData(t, testPool)
	repo := NewOrderRepository(testPool)

	createDto := dto.CreateOrderDTO{Name: "Order to delete", StatusID: statusID, ProretyID: proretyID, ExecutorID: executorID, Massage: "..."}
	newID, err := repo.CreateOrder(context.Background(), creatorID, createDto)
	require.NoError(t, err, "Подготовка теста SoftDeleteOrder: создание заявки не должно вызывать ошибок")

	err = repo.SoftDeleteOrder(context.Background(), uint64(newID))
	require.NoError(t, err)

	_, err = repo.FindOrder(context.Background(), uint64(newID))
	assert.ErrorIs(t, err, apperrors.ErrNotFound, "Заявка должна быть удалена")

	err = repo.SoftDeleteOrder(context.Background(), 99999)
	assert.ErrorIs(t, err, apperrors.ErrNotFound, "Должна быть ошибка NotFound для несуществующей заявки")
}

func TestOrderRepository_Integration_GetOrders(t *testing.T) {
	cleanupTables(t, testPool)
	creatorID, executorID, statusID, proretyID := seedData(t, testPool)
	repo := NewOrderRepository(testPool)

	for i := 0; i < 3; i++ {
		dto := dto.CreateOrderDTO{Name: "Get Order", StatusID: statusID, ProretyID: proretyID, ExecutorID: executorID, Massage: "..."}
		_, err := repo.CreateOrder(context.Background(), creatorID, dto)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	orders, total, err := repo.GetOrders(context.Background(), 2, 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), total, "Общее количество заявок должно быть 3")
	assert.Len(t, orders, 2, "Должно быть возвращено 2 заявки из-за лимита")
	assert.Equal(t, 2, orders[0].ID, "Первая заявка в результате должна иметь ID 2 из-за смещения")
}
