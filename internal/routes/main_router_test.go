// Файл: internal/routes/main_router_test.go
package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	// Ваши пакеты
	"request-system/internal/dto"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/customvalidator"
	"request-system/pkg/database/postgresql"
	"request-system/pkg/service"
	"request-system/pkg/utils"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// <<<--- ВОТ СТРУКТУРА, КОТОРАЯ БЫЛА ПОТЕРЯНА ---
// OrderTestSuite - это наш набор тестов
type OrderTestSuite struct {
	suite.Suite
	Echo           *echo.Echo
	DB             *pgxpool.Pool
	Redis          *redis.Client
	TestUserToken  string // Токен для пользователя с правами на создание заявок
	CreatedOrderID uint64 // ID созданной заявки
}

func (suite *OrderTestSuite) SetupSuite() {
	os.Setenv("DB_NAME", "request-system_test")
	os.Setenv("DB_USER", "postgres")
	os.Setenv("DB_PASSWORD", "postgres")

	e := echo.New()
	cfg := config.New()
	dbConn := postgresql.ConnectDB()
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 1}) // DB: 1 для тестов - это хорошо!

	v := validator.New()
	customvalidator.RegisterCustomValidations(v)
	e.Validator = utils.NewValidator(v)

	suite.Echo = e
	suite.DB = dbConn
	suite.Redis = redisClient

	// <<<--- НАЧАЛО ИСПРАВЛЕНИЙ ---

	// 1. Создаем заглушку логгера (zap.NewNop() - идеален для тестов, он ничего не пишет в консоль)
	nopLogger := zap.NewNop()

	// 2. Создаем структуру Loggers и заполняем все поля этой заглушкой.
	// В тестах нам не нужны разные файлы, так что один логгер-пустышка для всех - это то, что нужно.
	appLoggers := &Loggers{
		Main:         nopLogger,
		Auth:         nopLogger,
		Order:        nopLogger,
		User:         nopLogger,
		OrderHistory: nopLogger,
	}

	notificationService := services.NewMockNotificationService(nopLogger)
	userRepo := repositories.NewUserRepository(dbConn, nopLogger)
	statusRepo := repositories.NewStatusRepository(dbConn)
	permissionRepo := repositories.NewPermissionRepository(dbConn, nopLogger)
	cacheRepo := repositories.NewRedisCacheRepository(redisClient)
	authService := services.NewAuthService(userRepo, cacheRepo, nopLogger, &cfg.Auth, notificationService)
	authPermissionService := services.NewAuthPermissionService(permissionRepo, cacheRepo, nopLogger, 10*time.Minute)
	jwtSvc := service.NewJWTService(cfg.JWT.SecretKey, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL, nopLogger)

	InitRouter(e, dbConn, redisClient, jwtSvc, appLoggers, authPermissionService, cfg)
	ctx := context.Background()

	activeStatus, err := statusRepo.FindByCode(ctx, "ACTIVE")
	assert.NoError(suite.T(), err, "Статус с кодом 'ACTIVE' должен существовать в тестовой БД")
	assert.NotNil(suite.T(), activeStatus, "Найденный статус 'ACTIVE' не должен быть nil")

	testUserRoleID := uint64(3)

	cacheKey := fmt.Sprintf("role_permissions:%d", testUserRoleID)
	err = suite.Redis.Del(ctx, cacheKey).Err()
	assert.NoError(suite.T(), err, "Очистка кэша прав для роли не должна вызывать ошибок")

	permCreate, err := permissionRepo.FindPermissionByName(ctx, "order:create")
	assert.NoError(suite.T(), err, "Право 'order:create' должно существовать в тестовой БД")
	permView, err := permissionRepo.FindPermissionByName(ctx, "order:view")
	assert.NoError(suite.T(), err, "Право 'order:view' должно существовать в тестовой БД")
	permDelete, err := permissionRepo.FindPermissionByName(ctx, "order:delete")
	assert.NoError(suite.T(), err, "Право 'order:delete' должно существовать в тестовой БД")

	permissionsToLink := []uint64{permCreate.ID, permView.ID, permDelete.ID}
	for _, permID := range permissionsToLink {
		_, err = suite.DB.Exec(ctx,
			`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT (role_id, permission_id) DO NOTHING`,
			testUserRoleID,
			permID,
		)
		assert.NoError(suite.T(), err, "Привязка прав к роли не должна вызывать ошибок")
	}

	hashedPassword, _ := utils.HashPassword("password123")
	testUserEmail := fmt.Sprintf("testuser_%d@example.com", time.Now().UnixNano())
	randGenerator := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomNumber := randGenerator.Intn(1_000_000)
	phoneNumber := fmt.Sprintf("992987%06d", randomNumber)

	_, err = suite.DB.Exec(ctx,
		`INSERT INTO users (fio, email, phone_number, password, position, status_id, role_id, branch_id, department_id, must_change_password)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false)`,
		"Тестовый Пользователь",
		testUserEmail,
		phoneNumber,
		hashedPassword,
		"Тестировщик",
		activeStatus.ID,
		testUserRoleID,
		1, 1,
	)
	assert.NoError(suite.T(), err, "Создание тестового пользователя не должно вызывать ошибок")

	loginDTO := dto.LoginDTO{Login: testUserEmail, Password: "password123"}
	loggedInUser, err := authService.Login(ctx, loginDTO)
	assert.NoError(suite.T(), err, "Логин тестового пользователя не должен вызывать ошибок")
	assert.NotNil(suite.T(), loggedInUser)

	accessToken, _, err := jwtSvc.GenerateTokens(loggedInUser.ID, loggedInUser.RoleID, time.Hour, time.Hour)
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), accessToken)

	suite.TestUserToken = accessToken
}

func (suite *OrderTestSuite) TestOrderAPI() {
	var createdOrderID uint64

	// --- ШАГ 1: Создание Заявки ---
	suite.T().Run("CreateOrder should succeed", func(t *testing.T) {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		jsonData := `{"name": "Заявка для полного теста", "address": "Исходный адрес", "department_id": 1}`
		_ = writer.WriteField("data", jsonData)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/order", body)
		req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+suite.TestUserToken)
		rec := httptest.NewRecorder()

		suite.Echo.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code, "Ожидался статус 201 Created. Body: %s", rec.Body.String())

		var responseBody map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &responseBody)
		bodyData := responseBody["body"].(map[string]interface{})
		createdOrderID = uint64(bodyData["id"].(float64))
		assert.NotZero(t, createdOrderID)
	})

	suite.T().Logf("Создана заявка с ID: %d", createdOrderID)
	if createdOrderID == 0 {
		suite.T().Fatal("Не удалось создать заявку, дальнейшие тесты бессмысленны.")
	}

	// --- ШАГ 2: Обновление Заявки ---
	suite.T().Run("UpdateOrder should succeed", func(t *testing.T) {
		orderIDStr := strconv.FormatUint(createdOrderID, 10)

		updateBody := new(bytes.Buffer)
		updateWriter := multipart.NewWriter(updateBody)
		updateJsonData := `{"name": "Заявка ОБНОВЛЕНА", "priority_id": 3}`
		_ = updateWriter.WriteField("data", updateJsonData)
		updateWriter.Close()

		req := httptest.NewRequest(http.MethodPut, "/api/order/"+orderIDStr, updateBody)
		req.Header.Set(echo.HeaderContentType, updateWriter.FormDataContentType())
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+suite.TestUserToken)
		rec := httptest.NewRecorder()

		suite.Echo.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "Ожидался статус 200 OK при обновлении. Body: %s", rec.Body.String())

		// Проверяем, что в ответе вернулись обновленные данные
		var responseBody map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &responseBody)
		bodyData := responseBody["body"].(map[string]interface{})
		assert.Equal(t, "Заявка ОБНОВЛЕНА", bodyData["name"])
		assert.Equal(t, float64(3), bodyData["priority_id"]) // JSON числа всегда float64
	})

	// --- ШАГ 3: Получение и Удаление ---
	suite.T().Run("Get and Delete should work", func(t *testing.T) {
		orderIDStr := strconv.FormatUint(createdOrderID, 10)

		// 3.1 Проверяем, что заявка НАЙДЕНА и данные в ней действительно обновились
		reqGet := httptest.NewRequest(http.MethodGet, "/api/order/"+orderIDStr, nil)
		reqGet.Header.Set(echo.HeaderAuthorization, "Bearer "+suite.TestUserToken)
		recGet := httptest.NewRecorder()
		suite.Echo.ServeHTTP(recGet, reqGet)
		assert.Equal(t, http.StatusOK, recGet.Code)

		var responseBody map[string]interface{}
		json.Unmarshal(recGet.Body.Bytes(), &responseBody)
		bodyData := responseBody["body"].(map[string]interface{})
		assert.Equal(t, "Заявка ОБНОВЛЕНА", bodyData["name"], "Имя заявки в базе должно было обновиться")

		// 3.2 Удаляем заявку
		reqDel := httptest.NewRequest(http.MethodDelete, "/api/order/"+orderIDStr, nil)
		reqDel.Header.Set(echo.HeaderAuthorization, "Bearer "+suite.TestUserToken)
		recDel := httptest.NewRecorder()
		suite.Echo.ServeHTTP(recDel, reqDel)
		assert.Equal(t, http.StatusOK, recDel.Code)

		// 3.3 Убеждаемся, что теперь заявка не находится (возвращает 404)
		reqGetAfterDelete := httptest.NewRequest(http.MethodGet, "/api/order/"+orderIDStr, nil)
		reqGetAfterDelete.Header.Set(echo.HeaderAuthorization, "Bearer "+suite.TestUserToken)
		recGetAfterDelete := httptest.NewRecorder()
		suite.Echo.ServeHTTP(recGetAfterDelete, reqGetAfterDelete)
		assert.Equal(t, http.StatusNotFound, recGetAfterDelete.Code, "После удаления GET должен возвращать 404 Not Found")
	})
}

// TearDownSuite выполняется один раз после всех тестов, чтобы очистить ресурсы
func (suite *OrderTestSuite) TearDownSuite() {
	suite.DB.Close()
	suite.Redis.Close()
}

// TestFullOrderWorkflow - это наш главный тест, который проверяет весь жизненный цикл заявки
func (suite *OrderTestSuite) TestFullOrderWorkflow() {
	suite.Run("1_CreateOrder", func() {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)

		jsonData := `{"name": "Заявка созданная из автотеста", "address": "Адрес из автотеста", "department_id": 1}`
		_ = writer.WriteField("data", jsonData)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/order", body)
		req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+suite.TestUserToken)
		rec := httptest.NewRecorder()

		suite.Echo.ServeHTTP(rec, req)

		assert.Equal(suite.T(), http.StatusCreated, rec.Code, "Ожидался статус 201 Created")

		var responseBody map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &responseBody)

		bodyData := responseBody["body"].(map[string]interface{})
		idFloat := bodyData["id"].(float64)

		assert.Greater(suite.T(), idFloat, 0.0, "Должен быть возвращен ID созданной заявки")
		suite.CreatedOrderID = uint64(idFloat)
	})

	suite.Run("2_GetOrderByID", func() {
		assert.NotZero(suite.T(), suite.CreatedOrderID, "ID созданной заявки не должен быть 0")

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/order/%d", suite.CreatedOrderID), nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+suite.TestUserToken)
		rec := httptest.NewRecorder()
		suite.Echo.ServeHTTP(rec, req)

		assert.Equal(suite.T(), http.StatusOK, rec.Code, "Ожидался статус 200 OK")
	})

	suite.Run("3_DeleteOrder", func() {
		assert.NotZero(suite.T(), suite.CreatedOrderID, "ID созданной заявки не должен быть 0")

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/order/%d", suite.CreatedOrderID), nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+suite.TestUserToken)
		rec := httptest.NewRecorder()
		suite.Echo.ServeHTTP(rec, req)

		assert.Equal(suite.T(), http.StatusOK, rec.Code, "Ожидался статус 200 OK")

		reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/order/%d", suite.CreatedOrderID), nil)
		reqGet.Header.Set(echo.HeaderAuthorization, "Bearer "+suite.TestUserToken)
		recGet := httptest.NewRecorder()
		suite.Echo.ServeHTTP(recGet, reqGet)

		assert.Equal(suite.T(), http.StatusNotFound, recGet.Code, "После удаления заявка не должна быть найдена (ожидался статус 404)")
	})
}

// Эта стандартная функция Go запускает наш набор тестов
func TestOrderSuite(t *testing.T) {
	suite.Run(t, new(OrderTestSuite))
}
