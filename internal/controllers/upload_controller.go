package controllers

import (
	"net/http"

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

func NewUploadController(fs filestorage.FileStorageInterface, logger *zap.Logger) *UploadController {
	return &UploadController{fileStorage: fs, logger: logger}
}

func (ctrl *UploadController) Upload(c echo.Context) error {
	uploadContext := c.Param("context")
	rules, ok := utils.UploadContexts[uploadContext]
	if !ok {
		return utils.ErrorResponse(c, echo.NewHTTPError(http.StatusBadRequest, "Неизвестный контекст загрузки"))
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return utils.ErrorResponse(c, apperrors.ErrInternalServer)
	}

	if fileHeader.Size > (rules.MaxSizeMB * 1024 * 1024) {
		return utils.ErrorResponse(c, echo.NewHTTPError(http.StatusRequestEntityTooLarge, "Размер файла превышает допустимый лимит"))
	}

	src, err := fileHeader.Open()
	if err != nil {
		return utils.ErrorResponse(c, echo.NewHTTPError(http.StatusBadRequest, "Неверный контекст загрузки файла"))

	}
	defer src.Close() 

	buffer := make([]byte, 512)
	_, err = src.Read(buffer)
	if err != nil {
		return utils.ErrorResponse(c, apperrors.ErrInternalServer)
	}
	if _, err = src.Seek(0, 0); err != nil {
		return utils.ErrorResponse(c, apperrors.ErrInternalServer)
	}

	fileMimeType := http.DetectContentType(buffer)
	isAllowed := false
	for _, allowedType := range rules.AllowedMimeTypes {
		if fileMimeType == allowedType {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return utils.ErrorResponse(c, echo.NewHTTPError(http.StatusUnsupportedMediaType, "Недопустимый тип файла"))
	}

	savedPath, err := ctrl.fileStorage.Save(src, fileHeader.Filename)
	if err != nil {
		return utils.ErrorResponse(c, apperrors.ErrInternalServer)
	}

	fileURL := "/uploads/" + savedPath

	response := map[string]interface{}{
		"url":      fileURL,
		"filePath": savedPath,
	}

	return utils.SuccessResponse(c, response, "Файл успешно загружен", http.StatusOK)
}
