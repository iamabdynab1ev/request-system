package controllers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

type EquipImportController struct {
	importService *services.EquipImportService
	logger        *zap.Logger
}

func NewEquipImportController(importService *services.EquipImportService, logger *zap.Logger) *EquipImportController {
	return &EquipImportController{importService: importService, logger: logger}
}

func (c *EquipImportController) Import(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	permissionsMap, err := utils.GetPermissionsMapFromCtx(reqCtx)
	if err != nil || !authz.CanDo(authz.EquipmentsImport, authz.Context{Permissions: permissionsMap}) {
		return utils.ErrorResponse(ctx, apperrors.ErrForbidden, c.logger)
	}

	form, err := ctx.MultipartForm()
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("ошибка чтения формы: %w", err), c.logger)
	}

	files := form.File["files"]
	if len(files) == 0 {
		return ctx.JSON(http.StatusBadRequest, map[string]interface{}{
			"message": "Файлы не переданы. Отправьте поле 'files' с одним или несколькими .xlsx файлами",
		})
	}

	// Ограничиваем количество файлов, как вы и просили
	if len(files) > 3 {
		return ctx.JSON(http.StatusBadRequest, map[string]interface{}{
			"message": "Можно загрузить не более 3 файлов одновременно.",
		})
	}

	importFns := map[string]func(io.Reader) error{
		"atm":      c.importService.ImportAtmsReader,
		"terminal": c.importService.ImportTerminalsReader,
		"pos":      c.importService.ImportPosReader,
	}

	results := []map[string]interface{}{}
	hasError := false

	for _, fileHeader := range files {
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		if ext != ".xlsx" {
			results = append(results, map[string]interface{}{
				"file":    fileHeader.Filename,
				"success": false,
				"error":   "Только .xlsx файлы разрешены",
			})
			hasError = true
			continue
		}

		src, err := fileHeader.Open()
		if err != nil {
			results = append(results, map[string]interface{}{
				"file":    fileHeader.Filename,
				"success": false,
				"error":   "Ошибка открытия файла",
			})
			hasError = true
			continue
		}
		defer src.Close()

		// Ограничиваем чтение файла, например, 5 мегабайтами, чтобы избежать OOM.
		const maxFileSize = 5 * 1024 * 1024                 // 5 MB
		limitedReader := io.LimitReader(src, maxFileSize+1) // Читаем на 1 байт больше, чтобы проверить превышение

		buf, err := io.ReadAll(limitedReader)
		if err != nil {
			results = append(results, map[string]interface{}{
				"file":    fileHeader.Filename,
				"success": false,
				"error":   "Ошибка чтения файла",
			})
			hasError = true
			continue
		}

		if len(buf) > maxFileSize {
			errorMsg := fmt.Sprintf("Файл '%s' слишком большой. Максимальный размер: %d MB.", fileHeader.Filename, maxFileSize/1024/1024)
			return ctx.JSON(http.StatusBadRequest, map[string]interface{}{"message": errorMsg})
		}

		if err != nil {
			results = append(results, map[string]interface{}{
				"file":    fileHeader.Filename,
				"success": false,
				"error":   "Ошибка чтения файла",
			})
			hasError = true
			continue
		}

		// Определяем тип файла
		fileType, err := c.importService.DetectFileTypeReader(bytes.NewReader(buf))
		if err != nil {
			results = append(results, map[string]interface{}{
				"file":    fileHeader.Filename,
				"success": false,
				"error":   err.Error(),
			})
			hasError = true
			continue
		}

		// Запускаем импорт
		if err := importFns[fileType](bytes.NewReader(buf)); err != nil {
			c.logger.Error("Ошибка импорта", zap.String("type", fileType), zap.Error(err))
			results = append(results, map[string]interface{}{
				"file":    fileHeader.Filename,
				"type":    fileType,
				"success": false,
				"error":   err.Error(),
			})
			hasError = true
		} else {
			results = append(results, map[string]interface{}{
				"file":    fileHeader.Filename,
				"type":    fileType,
				"success": true,
			})
		}
	}

	status := http.StatusOK
	if hasError {
		status = http.StatusMultiStatus
	}

	return ctx.JSON(status, map[string]interface{}{
		"message": "Импорт завершён",
		"results": results,
	})
}
