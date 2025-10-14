package controllers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"

	"request-system/internal/entities"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

type ReportController struct {
	reportService services.ReportServiceInterface
	logger        *zap.Logger
}

func NewReportController(reportService services.ReportServiceInterface, logger *zap.Logger) *ReportController {
	return &ReportController{
		reportService: reportService,
		logger:        logger,
	}
}

func (c *ReportController) GetHistoryReport(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	standardFilter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	reportFilter, err := c.parseReportSpecificFilters(ctx, standardFilter)
	if err != nil {
		c.logger.Warn("GetHistoryReport: invalid filter parameters", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	c.logger.Debug("Parsed report filters", zap.Any("filters", reportFilter))

	reportData, totalCount, err := c.reportService.GetHistoryReport(reqCtx, *reportFilter)
	if err != nil {
		c.logger.Error("GetHistoryReport: service returned an error", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	format := strings.ToLower(ctx.QueryParam("format"))
	switch format {
	case "csv", "xlsx":
		reportFilter.Page = 1
		reportFilter.PerPage = 10000
		fullReportData, _, err := c.reportService.GetHistoryReport(reqCtx, *reportFilter)
		if err != nil {
			return utils.ErrorResponse(ctx, err, c.logger)
		}
		if format == "csv" {
			return c.respondWithCSV(ctx, fullReportData)
		}
		return c.respondWithXLSX(ctx, fullReportData)
	default:
		return utils.SuccessResponse(ctx, reportData, "Отчет успешно сформирован", http.StatusOK, totalCount)
	}
}

func (c *ReportController) parseReportSpecificFilters(ctx echo.Context, f types.Filter) (*entities.ReportFilter, error) {
	reportFilter := &entities.ReportFilter{
		Page:    f.Page,
		PerPage: f.Limit,
	}

	var orderIDsStr []string
	if ids, ok := ctx.Request().URL.Query()["order_ids[]"]; ok && len(ids) > 0 {
		orderIDsStr = ids
	} else if idStr := ctx.QueryParam("order_ids"); idStr != "" {
		orderIDsStr = strings.Split(idStr, ",")
	}
	if len(orderIDsStr) > 0 {
		ids, err := utils.ParseUint64Slice(orderIDsStr)
		if err != nil {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат order_ids", err, nil)
		}
		reportFilter.OrderIDs = ids
	}

	var userIDsStr []string
	if ids, ok := ctx.Request().URL.Query()["user_ids[]"]; ok && len(ids) > 0 {
		userIDsStr = ids
	} else if idStr := ctx.QueryParam("user_ids"); idStr != "" {
		userIDsStr = strings.Split(idStr, ",")
	}
	if len(userIDsStr) > 0 {
		ids, err := utils.ParseUint64Slice(userIDsStr)
		if err != nil {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат user_ids", err, nil)
		}
		reportFilter.UserIDs = ids
	}

	if eventTypes := ctx.Request().URL.Query()["event_types[]"]; len(eventTypes) > 0 {
		reportFilter.EventTypes = eventTypes
	}

	if dateFromStr := ctx.QueryParam("date_from"); dateFromStr != "" {
		t, err := time.Parse(time.RFC3339, dateFromStr)
		if err != nil {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат date_from", err, nil)
		}
		reportFilter.DateFrom = &t
	}
	if dateToStr := ctx.QueryParam("date_to"); dateToStr != "" {
		t, err := time.Parse(time.RFC3339, dateToStr)
		if err != nil {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат date_to", err, nil)
		}
		reportFilter.DateTo = &t
	}

	reportFilter.MetadataJSON = ctx.QueryParam("metadata_filter")

	sortOrder := strings.ToLower(ctx.QueryParam("sort_order"))
	if sortOrder == "asc" {
		reportFilter.SortOrder = "asc"
	} else {
		reportFilter.SortOrder = "desc"
	}

	return reportFilter, nil
}

func (c *ReportController) respondWithCSV(ctx echo.Context, data []entities.HistoryReportItem) error {
	fileName := fmt.Sprintf("report_%s.csv", time.Now().Format("2006-01-02"))
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename="+fileName)
	ctx.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")
	ctx.Response().Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(ctx.Response().Writer)

	header := []string{
		"ID События", "ID Заявки", "Название заявки", "ID Пользователя", "Имя пользователя",
		"Тип события", "Старое значение", "Новое значение", "Комментарий",
		"Доп. данные (JSON)", "Время события",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, item := range data {
		row := []string{
			strconv.FormatUint(item.ID, 10),
			strconv.FormatUint(item.OrderID, 10),
			item.OrderName,
			strconv.FormatUint(item.UserID, 10),
			item.UserName,
			item.EventType,
			item.OldValue.String,
			item.NewValue.String,
			item.Comment.String,
			string(item.Metadata),
			item.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	writer.Flush()
	return nil
}

// controllers/report_controller.go

// controllers/report_controller.go

func (c *ReportController) respondWithXLSX(ctx echo.Context, data []entities.HistoryReportItem) error {
	f := excelize.NewFile()
	defer f.Close()

	// --- 1. ЗАПИСЫВАЕМ СЫРЫЕ ДАННЫЕ НА ЛИСТ "Report Data" ---
	sheetNameData := "Report Data"
	f.SetSheetName("Sheet1", sheetNameData)

	// Заголовки основной таблицы
	headers := []string{
		"ID События", "ID Заявки", "Название заявки", "ID Пользователя", "Имя пользователя",
		"Тип события", "Старое значение", "Новое значение", "Комментарий",
		"Доп. данные (JSON)", "Время события",
	}
	if err := f.SetSheetRow(sheetNameData, "A1", &headers); err != nil {
		return err // Возвращаем ошибку, если не удалось записать заголовки
	}

	// Стиль для заголовков
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	f.SetCellStyle(sheetNameData, "A1", "K1", style)

	// Заполняем основную таблицу данными
	for i, item := range data {
		row := []interface{}{
			item.ID, item.OrderID, item.OrderName, item.UserID, item.UserName,
			item.EventType, item.OldValue.String, item.NewValue.String,
			item.Comment.String, string(item.Metadata),
			item.CreatedAt,
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := f.SetSheetRow(sheetNameData, cell, &row); err != nil {
			return err
		}
	}
	// Устанавливаем ширину колонок для основной таблицы
	f.SetColWidth(sheetNameData, "C", "C", 40) // Name
	f.SetColWidth(sheetNameData, "E", "I", 25)
	f.SetColWidth(sheetNameData, "J", "J", 40) // Metadata
	f.SetColWidth(sheetNameData, "K", "K", 20) // CreatedAt

	// --- 2. ПОДГОТАВЛИВАЕМ ДАННЫЕ ДЛЯ ДИАГРАММЫ ---
	eventsCount := make(map[string]int)
	for _, item := range data {
		eventsCount[item.EventType]++
	}

	if len(eventsCount) > 0 {
		sheetNameChart := "Chart Data"
		f.NewSheet(sheetNameChart)

		f.SetCellValue(sheetNameChart, "A1", "Тип события")
		f.SetCellValue(sheetNameChart, "B1", "Количество")

		rowNum := 2
		for eventType, count := range eventsCount {
			f.SetCellValue(sheetNameChart, fmt.Sprintf("A%d", rowNum), eventType)
			f.SetCellValue(sheetNameChart, fmt.Sprintf("B%d", rowNum), count)
			rowNum++
		}
		// Можно скрыть этот лист от пользователя, чтобы он видел только диаграмму
		f.SetSheetVisible(sheetNameChart, false)

		// --- 3. СОЗДАЕМ И ВСТАВЛЯЕМ ДИАГРАММУ ---
		lastDataRow := rowNum - 1
		chartOptions := excelize.Chart{
			Type: excelize.Pie,
			Series: []excelize.ChartSeries{
				{
					Name:       fmt.Sprintf("'%s'!$B$1", sheetNameChart),
					Categories: fmt.Sprintf("'%s'!$A$2:$A$%d", sheetNameChart, lastDataRow),
					Values:     fmt.Sprintf("'%s'!$B$2:$B$%d", sheetNameChart, lastDataRow),
				},
			},
			Title: []excelize.RichTextRun{
				{Text: "Распределение событий по типам"},
			},
			Legend:   excelize.ChartLegend{Position: "right"},
			PlotArea: excelize.ChartPlotArea{ShowPercent: true},
		}

		// Вставляем диаграмму на основной лист
		if err := f.AddChart(sheetNameData, "M2", &chartOptions); err != nil {
			c.logger.Error("Could not add chart to XLSX report", zap.Error(err))
			// Не возвращаем ошибку, так как отчет без диаграммы тоже ценен
		}
	}

	// --- 4. ОТДАЕМ ФАЙЛ ПОЛЬЗОВАТЕЛЮ ---
	fileName := fmt.Sprintf("report_with_chart_%s.xlsx", time.Now().Format("2006-01-02"))

	// Сначала говорим, что будем писать бинарные данные
	ctx.Response().Header().Set(echo.HeaderContentType, echo.MIMEOctetStream)
	// Потом говорим, как назвать файл
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename="+fileName)
	// И еще раз явно задаем правильный MIME-тип
	ctx.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	ctx.Response().WriteHeader(http.StatusOK) // Явно говорим, что ответ успешный

	return f.Write(ctx.Response().Writer)
}
