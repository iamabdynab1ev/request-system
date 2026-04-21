package services

import (
	"context"
	"database/sql"
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/events"
	"request-system/internal/repositories"
	"request-system/pkg/utils"
)

func (s *OrderService) detectAndLogChanges(ctx context.Context, tx pgx.Tx, old, new *entities.Order, dto dto.UpdateOrderDTO, actor *entities.User, txID uuid.UUID, now time.Time) (bool, error) {
	hasLoggable := false

	if dto.Comment != nil && strings.TrimSpace(*dto.Comment) != "" {
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "COMMENT", nil, nil, dto.Comment, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if old.Name != new.Name {
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "NAME_CHANGE", &new.Name, &old.Name, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}
	if !utils.StringPtrEqual(old.Address, new.Address) {
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "ADDRESS_CHANGE", new.Address, old.Address, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if !utils.TimeEqual(old.Duration, new.Duration) {
		valNew := ""
		valOld := ""
		if new.Duration != nil {
			valNew = new.Duration.Format(time.RFC3339)
		}
		if old.Duration != nil {
			valOld = old.Duration.Format(time.RFC3339)
		}

		if valNew != "" || valOld != "" {
			newPtr := &valNew
			oldPtr := &valOld
			if valNew == "" {
				newPtr = nil
			}
			if valOld == "" {
				oldPtr = nil
			}
			if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "DURATION_CHANGE", newPtr, oldPtr, nil, txID, *new); err != nil {
				return false, err
			}
			hasLoggable = true
		}
	}

	if utils.DiffPtr(old.EquipmentID, new.EquipmentID) {
		valNew := utils.PtrToString(new.EquipmentID)
		valOld := utils.PtrToString(old.EquipmentID)
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "EQUIPMENT_CHANGE", &valNew, &valOld, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if utils.DiffPtr(old.EquipmentTypeID, new.EquipmentTypeID) {
		valNew := utils.PtrToString(new.EquipmentTypeID)
		valOld := utils.PtrToString(old.EquipmentTypeID)
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "EQUIPMENT_TYPE_CHANGE", &valNew, &valOld, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if utils.DiffPtr(old.OrderTypeID, new.OrderTypeID) {
		valNew := utils.PtrToString(new.OrderTypeID)
		valOld := utils.PtrToString(old.OrderTypeID)
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "ORDER_TYPE_CHANGE", &valNew, &valOld, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if utils.DiffPtr(old.ExecutorID, new.ExecutorID) {
		newExName := s.resolveUserName(ctx, new.ExecutorID)
		txt := "Назначено на: " + newExName
		valNew := utils.PtrToString(new.ExecutorID)
		valOld := utils.PtrToString(old.ExecutorID)

		item := &repositories.OrderHistoryItem{
			OrderID: new.ID, UserID: actor.ID, EventType: "DELEGATION",
			OldValue: s.toNullStr(valOld), NewValue: s.toNullStr(valNew),
			Comment: s.toNullStr(txt), TxID: &txID, CreatedAt: now,
			ExecutorFio:  s.toNullStr(newExName),
			DelegatorFio: s.toNullStr(actor.Fio),
		}
		if err := s.addHistoryAndPublish(ctx, tx, item, *new, actor); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if old.StatusID != new.StatusID {
		sStrOld := fmt.Sprintf("%d", old.StatusID)
		sStrNew := fmt.Sprintf("%d", new.StatusID)
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "STATUS_CHANGE", &sStrNew, &sStrOld, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if utils.DiffPtr(old.PriorityID, new.PriorityID) {
		valNew := utils.PtrToString(new.PriorityID)
		valOld := utils.PtrToString(old.PriorityID)
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "PRIORITY_CHANGE", &valNew, &valOld, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if utils.DiffPtr(old.DepartmentID, new.DepartmentID) ||
		utils.DiffPtr(old.OtdelID, new.OtdelID) ||
		utils.DiffPtr(old.BranchID, new.BranchID) ||
		utils.DiffPtr(old.OfficeID, new.OfficeID) {

		changes := []string{}
		if utils.DiffPtr(old.DepartmentID, new.DepartmentID) {
			changes = append(changes, fmt.Sprintf("department_id: %s → %s", utils.PtrToString(old.DepartmentID), utils.PtrToString(new.DepartmentID)))
		}
		if utils.DiffPtr(old.OtdelID, new.OtdelID) {
			changes = append(changes, fmt.Sprintf("otdel_id: %s → %s", utils.PtrToString(old.OtdelID), utils.PtrToString(new.OtdelID)))
		}
		if utils.DiffPtr(old.BranchID, new.BranchID) {
			changes = append(changes, fmt.Sprintf("branch_id: %s → %s", utils.PtrToString(old.BranchID), utils.PtrToString(new.BranchID)))
		}
		if utils.DiffPtr(old.OfficeID, new.OfficeID) {
			changes = append(changes, fmt.Sprintf("office_id: %s → %s", utils.PtrToString(old.OfficeID), utils.PtrToString(new.OfficeID)))
		}

		txt := "Смена структуры: " + strings.Join(changes, "; ")

		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "STRUCTURE_CHANGE", nil, nil, &txt, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}
	return hasLoggable, nil
}

func (s *OrderService) attachFileToOrderInTx(ctx context.Context, tx pgx.Tx, orderID, userID uint64, file *multipart.FileHeader, txID *uuid.UUID, order *entities.Order) (uint64, error) {
	reader, err := file.Open()
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	filePath, err := s.fileStorage.Save(reader, file.Filename, "orders")
	if err != nil {
		return 0, err
	}

	attach := &entities.Attachment{
		OrderID: orderID, UserID: userID, FileName: file.Filename, FilePath: filePath,
		FileType: file.Header.Get("Content-Type"), FileSize: file.Size, CreatedAt: time.Now(),
	}
	id, err := s.attachRepo.CreateInTx(ctx, tx, attach)
	if err != nil {
		return 0, err
	}

	actor, _ := s.userRepo.FindUserByIDInTx(ctx, tx, userID)

	historyItem := &repositories.OrderHistoryItem{
		OrderID: orderID, UserID: userID, EventType: "ATTACHMENT_ADD",
		NewValue: s.toNullStr(file.Filename), Attachment: attach, AttachmentID: sql.NullInt64{Int64: int64(id), Valid: true},
		TxID: txID, CreatedAt: time.Now(), CreatorFio: s.toNullStr(actor.Fio),
	}
	if err := s.addHistoryAndPublish(ctx, tx, historyItem, *order, actor); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *OrderService) addHistoryAndPublish(ctx context.Context, tx pgx.Tx, item *repositories.OrderHistoryItem, o entities.Order, a *entities.User) error {
	if err := s.historyRepo.CreateInTx(ctx, tx, item); err != nil {
		return err
	}
	s.eventBus.Publish(ctx, events.OrderHistoryCreatedEvent{HistoryItem: *item, Order: &o, Actor: a})
	return nil
}

func (s *OrderService) logHistoryEvent(ctx context.Context, tx pgx.Tx, oid uint64, actor *entities.User, evtType string, newVal, oldVal, comment *string, txID uuid.UUID, ord entities.Order) error {
	item := &repositories.OrderHistoryItem{
		OrderID: oid, UserID: actor.ID, EventType: evtType,
		NewValue: s.toNullStrPtr(newVal), OldValue: s.toNullStrPtr(oldVal),
		Comment:    s.toNullStrPtr(comment),
		TxID:       &txID,
		CreatedAt:  time.Now(),
		CreatorFio: s.toNullStr(actor.Fio),
	}
	return s.addHistoryAndPublish(ctx, tx, item, ord, actor)
}

func (s *OrderService) resolveUserName(ctx context.Context, uid *uint64) string {
	if uid == nil {
		return ""
	}
	u, _ := s.userRepo.FindUserByID(ctx, *uid)
	if u != nil {
		return u.Fio
	}
	return ""
}

func (s *OrderService) toNullStr(v string) sql.NullString {
	return sql.NullString{String: v, Valid: true}
}

func (s *OrderService) toNullStrPtr(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func timePointersEqual(t1, t2 *time.Time) bool {
	if t1 == nil && t2 == nil {
		return true
	}
	if t1 == nil || t2 == nil {
		return false
	}
	return t1.Equal(*t2)
}
