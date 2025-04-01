package controllers
import (
	"net/http"

	"request-system/pkg/utils"
	"github.com/labstack/echo/v4"
)

type EquipmentController struct {}

func NewEquipmentController() *EquipmentController {
	return &EquipmentController{}
}


func (c *EquipmentController) GetEquipments(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *EquipmentController) FindEquipments(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *EquipmentController) CreateEquipments(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *EquipmentController) UpdateEquipments(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}

func (c *EquipmentController) DeleteEquipments(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


