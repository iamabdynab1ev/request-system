package main

import (
	"log"

	"request-system/internal/routes"
	"request-system/pkg/database/postgresql"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	var dbConn = postgresql.ConnectDB()

	if dbConn == nil {
		panic("Ошибка подключения к БД")
	}

	routes.INIT_ROUTER(e, dbConn)

	log.Println("🚀 Сервер запущен на :8080")
	e.Logger.Fatal(e.Start(":8080"))
}
