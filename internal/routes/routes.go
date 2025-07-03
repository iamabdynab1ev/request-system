package routes

import (
	"request-system/internal/repositories"
	mid "request-system/pkg/middleware"
	"request-system/pkg/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func INIT_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool, jwtSvc service.JWTService, logger *zap.Logger) {
	logger.Info("INIT_ROUTER: Начало создания маршрутов")

	commentRepo := repositories.NewOrderCommentRepository(dbConn)
	delegationRepo := repositories.NewOrderDelegationRepository(dbConn)
	userRepo := repositories.NewUserRepository(dbConn)
	statusRepo := repositories.NewStatusRepository(dbConn)
	proretyRepo := repositories.NewProretyRepository(dbConn)

	authMW := mid.NewAuthMiddleware(jwtSvc, userRepo)

	RUN_STATUS_ROUTER(e, dbConn)
	RUN_PRORETY_ROUTER(e, dbConn)
	RUN_DEPARTMENT_ROUTER(e, dbConn)
	RUN_OTDEL_ROUTER(e, dbConn)
	RUN_BRANCH_ROUTER(e, dbConn)
	RUN_OFFICE_ROUTER(e, dbConn)
	RUN_PERMISSION_ROUTER(e, dbConn)
	RUN_ROLE_ROUTER(e, dbConn)
	RUN_EQUIPMENT_ROUTER(e, dbConn)
	RUN_USER_ROUTER(e, dbConn)
	RUN_EQUIPMENT_TYPE_ROUTER(e, dbConn)
	RUN_ROLE_PERMISSION_ROUTER(e, dbConn)
	RUN_ORDER_DOCUMENT_ROUTER(e, dbConn)
	RUN_POSITION_ROUTER(e, dbConn)

	RUN_AUTH_ROUTER(e, dbConn, jwtSvc, logger)

	RUN_ORDER_DELEGATION_ROUTER(e, dbConn, authMW, logger)
	RUN_ORDER_COMMENT_ROUTER(e, dbConn, authMW, logger)
	RUN_ORDER_ROUTER(e, dbConn, commentRepo, delegationRepo, userRepo, statusRepo, proretyRepo, authMW, logger)

	logger.Info("INIT_ROUTER: Создание маршрутов завершено")
}
