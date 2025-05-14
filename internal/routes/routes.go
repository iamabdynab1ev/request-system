package routes

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func INIT_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	RUN_STATUS_ROUTER(e, dbConn)
	RUN_PRORETY_ROUTER(e)
	RUN_DEPARTMENT_ROUTER(e)
	RUN_OTDEL_ROUTER(e)
	RUN_BRANCH_ROUTER(e, dbConn)
	RUN_OFFICE_ROUTER(e)
	RUN_PERMISSION_ROUTER(e)
	RUN_ROLE_ROUTER(e, dbConn)
	RUN_EQUIPMENT_ROUTER(e)
	RUN_USER_ROUTER(e)
	RUN_ORDER_ROUTER(e)
	
}
