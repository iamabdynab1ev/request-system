package controllers
import (
	"net/http"

	"request-system/pkg/utils"
	"github.com/labstack/echo/v4"
)

type DepartmentController struct {}

func NewDepartmentController() *DepartmentController {
	return &DepartmentController{}
}


func (c *DepartmentController) GetDepartments(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


func (c *DepartmentController) FindDepartments(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


func (c *DepartmentController) CreateDepartments(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


func (c *DepartmentController) UpdateDepartments(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}

func (c *DepartmentController) DeleteDepartments(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}


