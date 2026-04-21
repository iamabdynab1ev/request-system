package services

import (
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
)

func TestValidateCreateFieldPermissions_RejectsForbiddenField(t *testing.T) {
	service := &OrderService{}
	authCtx := &authz.Context{
		Permissions: map[string]bool{
			authz.OrdersCreate: true,
		},
	}

	createDTO := dto.CreateOrderDTO{
		Name:         "Новая заявка",
		OrderTypeID:  uint64Ptr(10),
		DepartmentID: uint64Ptr(15),
	}

	err := service.validateCreateFieldPermissions(authCtx, createDTO, nil)
	if err == nil {
		t.Fatal("expected forbidden error for missing field permission")
	}

	httpErr, ok := err.(interface{ StatusCode() int })
	if ok && httpErr.StatusCode() != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", httpErr.StatusCode())
	}
}

func TestValidateCreateFieldPermissions_AllowsPermittedFields(t *testing.T) {
	service := &OrderService{}
	authCtx := &authz.Context{
		Permissions: map[string]bool{
			authz.OrdersCreateName:         true,
			authz.OrdersCreateDepartmentID: true,
			authz.OrdersCreatePriorityID:   true,
		},
	}

	createDTO := dto.CreateOrderDTO{
		Name:         "Новая заявка",
		OrderTypeID:  uint64Ptr(10),
		DepartmentID: uint64Ptr(15),
		PriorityID:   uint64Ptr(2),
	}

	if err := service.validateCreateFieldPermissions(authCtx, createDTO, nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateCreateFieldPermissions_RejectsForbiddenFile(t *testing.T) {
	service := &OrderService{}
	authCtx := &authz.Context{
		Permissions: map[string]bool{
			authz.OrdersCreateName: true,
		},
	}

	createDTO := dto.CreateOrderDTO{
		Name:        "Новая заявка",
		OrderTypeID: uint64Ptr(10),
	}

	file := multipartFileHeaderStub()
	err := service.validateCreateFieldPermissions(authCtx, createDTO, file)
	if err == nil {
		t.Fatal("expected forbidden error for file permission")
	}
}

func TestDashboardSummaryAffected(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	oldOrder := &entities.Order{
		StatusID:                 1,
		PriorityID:               uint64Ptr(2),
		ExecutorID:               uint64Ptr(3),
		DepartmentID:             uint64Ptr(4),
		OrderTypeID:              uint64Ptr(5),
		Duration:                 &now,
		ResolutionTimeSeconds:    uint64Ptr(60),
		FirstResponseTimeSeconds: uint64Ptr(10),
	}
	newOrder := &entities.Order{
		StatusID:                 1,
		PriorityID:               uint64Ptr(2),
		ExecutorID:               uint64Ptr(3),
		DepartmentID:             uint64Ptr(4),
		OrderTypeID:              uint64Ptr(5),
		Duration:                 &now,
		ResolutionTimeSeconds:    uint64Ptr(60),
		FirstResponseTimeSeconds: uint64Ptr(10),
	}

	if dashboardSummaryAffected(oldOrder, newOrder) {
		t.Fatal("expected identical orders not to affect dashboard summary")
	}

	newOrder.StatusID = 2
	if !dashboardSummaryAffected(oldOrder, newOrder) {
		t.Fatal("expected status change to affect dashboard summary")
	}
}

func uint64Ptr(v uint64) *uint64 { return &v }

func multipartFileHeaderStub() *multipart.FileHeader {
	return &multipart.FileHeader{Filename: "test.txt"}
}
