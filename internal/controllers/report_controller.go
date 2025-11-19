package controllers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"

	"request-system/internal/entities"
	"request-system/internal/services"
	"request-system/pkg/utils"
)

type ReportController struct {
	reportService services.ReportServiceInterface
	logger        *zap.Logger
}

func NewReportController(reportService services.ReportServiceInterface, logger *zap.Logger) *ReportController {
	return &ReportController{reportService: reportService, logger: logger}
}

func (c *ReportController) GetReport(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter, format := c.parseFilters(ctx)
	c.logger.Debug("Запрос на отчет с фильтрами", zap.Any("filters", filter), zap.String("format", format))

	if format == "xlsx" {
		data, _, err := c.reportService.GetReportForExcel(reqCtx, filter)
		if err != nil {
			return utils.ErrorResponse(ctx, err, c.logger)
		}
		return c.respondWithXLSX(ctx, data)
	}
	dtos, total, err := c.reportService.GetReportDTOs(reqCtx, filter)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, dtos, "Отчет успешно сформирован", http.StatusOK, total)
}

func (c *ReportController) parseFilters(ctx echo.Context) (entities.ReportFilter, string) {
	stdFilter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	filter := entities.ReportFilter{
		Page:    stdFilter.Page,
		PerPage: stdFilter.Limit,
	}
	format := strings.ToLower(ctx.QueryParam("format"))

	if format == "xlsx" {
		filter.Page = 1
		filter.PerPage = 100000 // Выгружаем все для экспорта
	}

	if df := ctx.QueryParam("date_from"); df != "" {
		if t, err := time.Parse(time.RFC3339, df); err == nil {
			filter.DateFrom = &t
		}
	}
	if dt := ctx.QueryParam("date_to"); dt != "" {
		if t, err := time.Parse(time.RFC3339, dt); err == nil {
			filter.DateTo = &t
		}
	}

	parseIDs := func(name string) []uint64 {
		var strs []string
		if arr, ok := ctx.QueryParams()[name+"[]"]; ok {
			strs = arr
		} else if s := ctx.QueryParam(name); s != "" {
			strs = strings.Split(s, ",")
		}
		ids, _ := utils.ParseUint64Slice(strs)
		return ids
	}

	filter.ExecutorIDs = parseIDs("executor_ids")
	filter.OrderTypeIDs = parseIDs("order_type_ids")
	filter.PriorityIDs = parseIDs("priority_ids")

	return filter, format
}

var reportHeaders = []string{
	"№", "Заявитель", "Дата обращения", "Время обращения", "ID заявки", "Категория", "Приоритет", "Статус",
	"Описание проблемы", "Ответственный", "Дата назначения", "Исполнитель", "Дата решения",
	"Время решения (часы)", "SLA (выполнен/нет)", "Источник", "Комментарий",
}

func rowToSlice(item entities.ReportItem) []interface{} {
	dateFmt, timeFmt := "02.01.2006", "15:04"

	nullStr := func(s sql.NullString) string {
		if s.Valid {
			return s.String
		}
		return ""
	}
	nullTime := func(t sql.NullTime, format string) string {
		if t.Valid {
			return t.Time.Format(format)
		}
		return ""
	}

	return []interface{}{
		item.OrderID,                        //
		nullStr(item.CreatorFio),            // Заявитель
		item.CreatedAt.Format(dateFmt),      // Дата обращения
		item.CreatedAt.Format(timeFmt),      // Время обращения
		item.OrderID,                        // ID заявки
		nullStr(item.OrderTypeName),         // Категория
		nullStr(item.PriorityName),          // Приоритет
		nullStr(item.StatusName),            // Статус
		nullStr(item.OrderName),             // Описание проблемы
		nullStr(item.ResponsibleFio),        // Ответственный
		nullTime(item.DelegatedAt, dateFmt), // Дата назначения
		nullStr(item.ExecutorFio),           // Исполнитель
		nullTime(item.CompletedAt, dateFmt), // Дата решения
		nullStr(item.ResolutionTimeStr),     // Время решения (часы)
		nullStr(item.SLAStatus),             // SLA
		nullStr(item.SourceDepartment),      // Источник
		nullStr(item.Comment),               // Комментарий
	}
}

func (c *ReportController) respondWithXLSX(ctx echo.Context, data []entities.ReportItem) error {
	f := excelize.NewFile()
	sheet := "Отчет по заявкам"
	f.SetSheetName("Sheet1", sheet)
	f.SetSheetRow(sheet, "A1", &reportHeaders)
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	f.SetCellStyle(sheet, "A1", "Q1", style)

	for i, item := range data {
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		row := rowToSlice(item)
		f.SetSheetRow(sheet, cell, &row)
	}
	// Авто-ширина колонок для красоты
	f.SetColWidth(sheet, "B", "B", 25)
	f.SetColWidth(sheet, "E", "H", 20)
	f.SetColWidth(sheet, "I", "I", 40)
	f.SetColWidth(sheet, "J", "L", 25)
	f.SetColWidth(sheet, "Q", "Q", 50)

	fileName := fmt.Sprintf("report_%s.xlsx", time.Now().Format("2006-01-02"))
	ctx.Response().Header().Set(echo.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename="+fileName)
	ctx.Response().WriteHeader(http.StatusOK)
	return f.Write(ctx.Response().Writer)
}
