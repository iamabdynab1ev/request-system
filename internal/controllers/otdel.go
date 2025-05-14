package controllers
import (
	"net/http"

	"request-system/pkg/utils"
	"github.com/labstack/echo/v4"
)

type OtdelController struct {}

func NewOtdelController() *OtdelController {
	return &OtdelController{}
}


func (c *OtdelController) GetOtdels(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


func (c *OtdelController) FindOtdels(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


func (c *OtdelController) CreateOtdels(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


func (c *OtdelController) UpdateOtdels(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}

func (c *OtdelController) DeleteOtdels(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


