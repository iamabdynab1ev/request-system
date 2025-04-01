package controllers
import (
	"net/http"

	"request-system/pkg/utils"
	"github.com/labstack/echo/v4"
)

type OfficeController struct {}

func NewOfficeController() *OfficeController {
	return &OfficeController{}
}


func (c *OfficeController) GetOffices(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *OfficeController) FindOffices(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *OfficeController) CreateOffices(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *OfficeController) UpdateOffices(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}

func (c *OfficeController) DeleteOffices(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


