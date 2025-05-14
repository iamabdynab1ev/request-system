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
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


func (c *EquipmentController) FindEquipments(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


func (c *EquipmentController) CreateEquipments(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


func (c *EquipmentController) UpdateEquipments(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}

func (c *EquipmentController) DeleteEquipments(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


