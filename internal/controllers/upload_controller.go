// controllers/upload_controller.go

package controllers

import (
	"net/http"

	"request-system/config"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type UploadController struct {
	fileStorage filestorage.FileStorageInterface
	logger      *zap.Logger
}

func (ctrl *UploadController) Upload(c echo.Context) error {
	uploadContext := c.Param("context")

	// Получаем правила из конфига
	rules, ok := config.UploadContexts[uploadContext]
	if !ok {
		return utils.ErrorResponse(c,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неизвестный контекст загрузки",
				apperrors.ErrBadRequest,
				map[string]interface{}{"context": uploadContext},
			),
			ctrl.logger,
		)
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return utils.ErrorResponse(c,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Файл не был передан",
				apperrors.ErrBadRequest,
				nil,
			),
			ctrl.logger,
		)
	}

	src, err := fileHeader.Open()
	if err != nil {
		return utils.ErrorResponse(c,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Ошибка обработки файла",
				apperrors.ErrBadRequest,
				nil,
			),
			ctrl.logger,
		)
	}
	defer src.Close()

	if err := utils.ValidateFile(fileHeader, src, uploadContext); err != nil {
		return utils.ErrorResponse(c,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				err.Error(),
				apperrors.ErrBadRequest,
				nil,
			),
			ctrl.logger,
		)
	}

	// Сохраняем файл
	savedPath, err := ctrl.fileStorage.Save(src, fileHeader.Filename, rules.PathPrefix)
	if err != nil {
		ctrl.logger.Error("Ошибка сохранения файла", zap.Error(err))
		return utils.ErrorResponse(c,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Ошибка сохранения файла",
				err,
				nil,
			),
			ctrl.logger,
		)
	}

	// Формируем URL
	fileURL := "/uploads/" + savedPath
	response := map[string]interface{}{
		"url":      fileURL,
		"filePath": savedPath,
	}

	return utils.SuccessResponse(c, response, "Файл успешно загружен", http.StatusOK)
}
