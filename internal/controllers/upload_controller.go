// controllers/upload_controller.go

package controllers

import (
	"net/http"

	"request-system/config"
	"request-system/pkg/filestorage"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type UploadController struct {
	fileStorage filestorage.FileStorageInterface
	logger      *zap.Logger
}

func NewUploadController(fs filestorage.FileStorageInterface, logger *zap.Logger) *UploadController {
	return &UploadController{fileStorage: fs, logger: logger}
}

func (ctrl *UploadController) Upload(c echo.Context) error {
	uploadContext := c.Param("context")

	// Получаем правила из конфига
	rules, ok := config.UploadContexts[uploadContext]
	if !ok {
		return utils.ErrorResponse(c, echo.NewHTTPError(http.StatusBadRequest, "Неизвестный контекст загрузки"))
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return utils.ErrorResponse(c, echo.NewHTTPError(http.StatusBadRequest, "Файл не был передан"))
	}

	src, err := fileHeader.Open()
	if err != nil {
		return utils.ErrorResponse(c, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обработки файла"))
	}
	defer src.Close()

	if err := utils.ValidateFile(fileHeader, src, uploadContext); err != nil {
		return utils.ErrorResponse(c, echo.NewHTTPError(http.StatusBadRequest, err.Error()))
	}

	// Передаем префикс из конфига в метод Save
	savedPath, err := ctrl.fileStorage.Save(src, fileHeader.Filename, rules.PathPrefix)
	if err != nil {
		ctrl.logger.Error("Ошибка сохранения файла", zap.Error(err))
		return utils.ErrorResponse(c, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка сохранения файла"))
	}

	// Теперь просто добавляем корневой `/uploads/` к пути, который вернул `Save`
	// `Save` вернет, например: "avatars/2024/05/20/file.jpg"
	// `fileURL` станет: "/uploads/avatars/2024/05/20/file.jpg"
	fileURL := "/uploads/" + savedPath

	response := map[string]interface{}{
		"url":      fileURL,
		"filePath": savedPath,
	}

	return utils.SuccessResponse(c, response, "Файл успешно загружен", http.StatusOK)
}
