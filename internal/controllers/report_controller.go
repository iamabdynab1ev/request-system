package controllers

import (
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

	data, total, err := c.reportService.GetReport(reqCtx, filter)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	if format == "xlsx" {
		return c.respondWithXLSX(ctx, data)
	}

	return utils.SuccessResponse(ctx, data, "Отчет успешно сформирован", http.StatusOK, total)
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
	var delegatedAt, completedAt, resHours string
	if item.DelegatedAt.Valid {
		delegatedAt = item.DelegatedAt.Time.Format(dateFmt + " " + timeFmt)
	}
	if item.CompletedAt.Valid {
		completedAt = item.CompletedAt.Time.Format(dateFmt)
	}
	if item.ResolutionHours.Valid {
		resHours = fmt.Sprintf("%.2f", item.ResolutionHours.Float64)
	}

	return []interface{}{
		item.OrderID, item.CreatorFio.String, item.CreatedAt.Format(dateFmt), item.CreatedAt.Format(timeFmt),
		item.OrderID, item.OrderTypeName.String, item.PriorityName.String, item.StatusName,
		item.OrderName, item.ExecutorFio.String, delegatedAt, item.ExecutorFio.String,
		completedAt, resHours, item.SLAStatus, "-", item.Comment.String,
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
