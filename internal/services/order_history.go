package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/utils"
)

type OrderHistoryServiceInterface interface {
	GetTimelineByOrderID(ctx context.Context, orderID uint64, limitStr, offsetStr string) ([]dto.TimelineEventDTO, error)
}

type historyUserLookup interface {
	FindUsersByIDs(ctx context.Context, userIDs []uint64) (map[uint64]entities.User, error)
	FindUserByID(ctx context.Context, id uint64) (*entities.User, error)
}

type historyDepartmentLookup interface {
	FindDepartment(ctx context.Context, id uint64) (*entities.Department, error)
}

type historyOtdelLookup interface {
	FindOtdel(ctx context.Context, id uint64) (*entities.Otdel, error)
}

type historyBranchLookup interface {
	FindBranch(ctx context.Context, id uint64) (*entities.Branch, error)
}

type historyOfficeLookup interface {
	FindOffice(ctx context.Context, id uint64) (*entities.Office, error)
}

type historyStatusLookup interface {
	FindStatus(ctx context.Context, id uint64) (*entities.Status, error)
}

type historyPriorityLookup interface {
	FindByID(ctx context.Context, id uint64) (*entities.Priority, error)
}

type OrderHistoryService struct {
	repo           repositories.OrderHistoryRepositoryInterface
	userRepo       historyUserLookup
	departmentRepo historyDepartmentLookup
	otdelRepo      historyOtdelLookup
	branchRepo     historyBranchLookup
	officeRepo     historyOfficeLookup
	statusRepo     historyStatusLookup
	priorityRepo   historyPriorityLookup
	logger         *zap.Logger
}

func NewOrderHistoryService(
	repo repositories.OrderHistoryRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	departmentRepo repositories.DepartmentRepositoryInterface,
	otdelRepo repositories.OtdelRepositoryInterface,
	branchRepo repositories.BranchRepositoryInterface,
	officeRepo repositories.OfficeRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface,
	logger *zap.Logger,
) OrderHistoryServiceInterface {
	return &OrderHistoryService{
		repo:           repo,
		userRepo:       userRepo,
		departmentRepo: departmentRepo,
		otdelRepo:      otdelRepo,
		branchRepo:     branchRepo,
		officeRepo:     officeRepo,
		statusRepo:     statusRepo,
		priorityRepo:   priorityRepo,
		logger:         logger,
	}
}

func (s *OrderHistoryService) GetTimelineByOrderID(ctx context.Context, orderID uint64, limitStr, offsetStr string) ([]dto.TimelineEventDTO, error) {
	// ПРОВЕРКА ПРАВ ДОСТУПА УДАЛЕНА ОТСЮДА.

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	historyEvents, err := s.repo.FindByOrderID(ctx, orderID, uint64(limit), uint64(offset))
	if err != nil {
		return []dto.TimelineEventDTO{}, err
	}
	if len(historyEvents) == 0 {
		return []dto.TimelineEventDTO{}, nil
	}

	meta := buildHistoryMetadata(historyEvents)
	resolver := newHistoryReferenceResolver(ctx, s, historyEvents, meta)

	timeline := make([]dto.TimelineEventDTO, 0, len(historyEvents))
	currentBlock := createTimelineBlock(historyEvents[0], resolver)
	addEventToBlock(currentBlock, historyEvents[0], resolver)

	for i := 1; i < len(historyEvents); i++ {
		event := historyEvents[i]
		prevEvent := historyEvents[i-1]

		isSameTransaction := event.TxID != nil && prevEvent.TxID != nil && event.TxID.String() == prevEvent.TxID.String()

		if isSameTransaction {
			addEventToBlock(currentBlock, event, resolver)
			continue
		}

		timeline = append(timeline, *currentBlock)
		currentBlock = createTimelineBlock(event, resolver)
		addEventToBlock(currentBlock, event, resolver)
	}

	timeline = append(timeline, *currentBlock)
	s.logger.Info("Таймлайн сформирован", zap.Int("blocks", len(timeline)), zap.Uint64("orderID", orderID))
	return timeline, nil
}

type historyMetadata struct {
	creatorID    uint64
	delegatorIDs map[uint64]struct{}
}

func buildHistoryMetadata(events []repositories.OrderHistoryItem) historyMetadata {
	meta := historyMetadata{delegatorIDs: make(map[uint64]struct{})}
	for _, e := range events {
		if e.EventType == "CREATE" {
			meta.creatorID = e.UserID
		}
		if e.EventType == "DELEGATION" {
			meta.delegatorIDs[e.UserID] = struct{}{}
		}
	}
	return meta
}

type historyReferenceResolver struct {
	ctx     context.Context
	service *OrderHistoryService
	meta    historyMetadata
	users   map[uint64]entities.User

	departmentNames map[uint64]string
	departmentSeen  map[uint64]bool
	otdelNames      map[uint64]string
	otdelSeen       map[uint64]bool
	branchNames     map[uint64]string
	branchSeen      map[uint64]bool
	officeNames     map[uint64]string
	officeSeen      map[uint64]bool
	statusNames     map[uint64]string
	statusSeen      map[uint64]bool
	priorityNames   map[uint64]string
	prioritySeen    map[uint64]bool
}

func newHistoryReferenceResolver(ctx context.Context, service *OrderHistoryService, events []repositories.OrderHistoryItem, meta historyMetadata) *historyReferenceResolver {
	resolver := &historyReferenceResolver{
		ctx:             ctx,
		service:         service,
		meta:            meta,
		users:           map[uint64]entities.User{},
		departmentNames: make(map[uint64]string),
		departmentSeen:  make(map[uint64]bool),
		otdelNames:      make(map[uint64]string),
		otdelSeen:       make(map[uint64]bool),
		branchNames:     make(map[uint64]string),
		branchSeen:      make(map[uint64]bool),
		officeNames:     make(map[uint64]string),
		officeSeen:      make(map[uint64]bool),
		statusNames:     make(map[uint64]string),
		statusSeen:      make(map[uint64]bool),
		priorityNames:   make(map[uint64]string),
		prioritySeen:    make(map[uint64]bool),
	}

	userIDs := uniqueHistoryUserIDs(events)
	if len(userIDs) == 0 {
		return resolver
	}

	users, err := service.userRepo.FindUsersByIDs(ctx, userIDs)
	if err != nil {
		service.logger.Warn("failed to preload history actors", zap.Error(err), zap.Int("users", len(userIDs)))
		return resolver
	}

	resolver.users = users
	return resolver
}

func uniqueHistoryUserIDs(events []repositories.OrderHistoryItem) []uint64 {
	seen := make(map[uint64]struct{}, len(events))
	ids := make([]uint64, 0, len(events))
	for _, event := range events {
		if _, ok := seen[event.UserID]; ok {
			continue
		}
		seen[event.UserID] = struct{}{}
		ids = append(ids, event.UserID)
	}
	return ids
}

// createTimelineBlock - хелпер для создания чистого блока таймлайна.
func createTimelineBlock(event repositories.OrderHistoryItem, resolver *historyReferenceResolver) *dto.TimelineEventDTO {
	return &dto.TimelineEventDTO{
		Lines:     []string{},
		Actor:     resolver.actorFromEvent(event),
		CreatedAt: event.CreatedAt.Format("02.01.2006 / 15:04"),
	}
}

func (r *historyReferenceResolver) actorFromEvent(event repositories.OrderHistoryItem) dto.ShortUserDTO {
	role := "executor"
	if event.UserID == r.meta.creatorID {
		role = "creator"
	} else if _, isDelegator := r.meta.delegatorIDs[event.UserID]; isDelegator {
		role = "delegator"
	}

	return dto.ShortUserDTO{ID: event.UserID, Fio: r.actorName(event), Role: role}
}

func (r *historyReferenceResolver) actorName(event repositories.OrderHistoryItem) string {
	if stored := historyActorNameFromEvent(event); stored != "" {
		return stored
	}
	if user, ok := r.users[event.UserID]; ok && strings.TrimSpace(user.Fio) != "" {
		return user.Fio
	}
	return "Неизвестный пользователь"
}

func historyActorNameFromEvent(event repositories.OrderHistoryItem) string {
	if event.EventType == "DELEGATION" {
		if name := strings.TrimSpace(utils.NullStringToString(event.DelegatorFio)); name != "" {
			return name
		}
	}

	for _, candidate := range []string{
		utils.NullStringToString(event.CreatorFio),
		utils.NullStringToString(event.DelegatorFio),
		utils.NullStringToString(event.ExecutorFio),
	} {
		if name := strings.TrimSpace(candidate); name != "" {
			return name
		}
	}

	return ""
}

// getActorFromEvent - теперь работает за O(1) благодаря заранее собранным мета-данным.
func getActorFromEvent(event repositories.OrderHistoryItem, s *OrderHistoryService, ctx context.Context, meta historyMetadata) dto.ShortUserDTO {
	user, err := s.userRepo.FindUserByID(ctx, event.UserID)
	fio := "Неизвестный пользователь"
	if err == nil && user != nil {
		fio = user.Fio
	}

	var role string
	if event.UserID == meta.creatorID {
		role = "creator"
	} else {
		if _, isDelegator := meta.delegatorIDs[event.UserID]; isDelegator {
			role = "delegator"
		} else {
			role = "executor"
		}
	}

	return dto.ShortUserDTO{ID: event.UserID, Fio: fio, Role: role}
}

func addEventToBlock(block *dto.TimelineEventDTO, event repositories.OrderHistoryItem, resolver *historyReferenceResolver) {
	if event.EventType == "COMMENT" {
		if comment := strings.TrimSpace(utils.NullStringToString(event.Comment)); comment != "" {
			block.Comment = &comment
		}
		return
	}

	if line := resolver.lineForEvent(block, event); line != "" {
		block.Lines = append(block.Lines, line)
	}
}

func (r *historyReferenceResolver) lineForEvent(block *dto.TimelineEventDTO, event repositories.OrderHistoryItem) string {
	newValue := utils.NullStringToString(event.NewValue)

	switch event.EventType {
	case "CREATE":
		if newValue == "" {
			return "Создана заявка"
		}
		return fmt.Sprintf("Создана заявка: «%s»", newValue)
	case "DELEGATION":
		return strings.TrimSpace(utils.NullStringToString(event.Comment))
	case "ATTACHMENT_ADD":
		if event.Attachment != nil {
			block.Attachment = &dto.AttachmentResponseDTO{
				ID:       event.Attachment.ID,
				FileName: event.Attachment.FileName,
				URL:      "/uploads/" + event.Attachment.FilePath,
			}
		}
		if newValue == "" {
			return "Прикреплен файл"
		}
		return fmt.Sprintf("Прикреплен файл: %s", newValue)
	case "DEPARTMENT_CHANGE":
		return r.departmentChangeLine(newValue)
	case "OTDEL_CHANGE":
		return r.otdelChangeLine(newValue)
	case "STATUS_CHANGE":
		return r.statusChangeLine(event, newValue)
	case "PRIORITY_CHANGE":
		return r.priorityChangeLine(newValue)
	case "DURATION_CHANGE":
		if parsedTime, err := time.Parse(time.RFC3339, newValue); err == nil {
			return fmt.Sprintf("Установлен срок выполнения до: %s", parsedTime.Format("02.01.2006 15:04"))
		}
		if newValue == "" {
			return ""
		}
		return fmt.Sprintf("Срок выполнения изменен на: %s", newValue)
	case "NAME_CHANGE":
		return fmt.Sprintf("Изменено название заявки: «%s» на «%s»", utils.NullStringToString(event.OldValue), newValue)
	case "ADDRESS_CHANGE":
		return fmt.Sprintf("Изменен адрес: «%s» на «%s»", utils.NullStringToString(event.OldValue), newValue)
	case "EQUIPMENT_CHANGE":
		if newValue == "" {
			return ""
		}
		return fmt.Sprintf("Изменено оборудование: ID на %s", newValue)
	case "EQUIPMENT_TYPE_CHANGE":
		if newValue == "" {
			return ""
		}
		return fmt.Sprintf("Изменен тип оборудования: ID на %s", newValue)
	case "ORDER_TYPE_CHANGE":
		if newValue == "" {
			return ""
		}
		return fmt.Sprintf("Изменен тип заявки: ID на %s", newValue)
	case "STRUCTURE_CHANGE":
		return r.structureChangeLine(strings.TrimSpace(utils.NullStringToString(event.Comment)))
	default:
		return ""
	}
}

func (r *historyReferenceResolver) departmentChangeLine(newValue string) string {
	id, err := strconv.ParseUint(newValue, 10, 64)
	if err != nil {
		if newValue == "" {
			return ""
		}
		return fmt.Sprintf("Изменено поле 'Департамент': ID на %s", newValue)
	}

	if name := r.departmentName(id); name != "" {
		return fmt.Sprintf("Заявка переведена в департамент: «%s»", name)
	}

	return fmt.Sprintf("Изменено поле 'Департамент': ID на %s", newValue)
}

func (r *historyReferenceResolver) otdelChangeLine(newValue string) string {
	id, err := strconv.ParseUint(newValue, 10, 64)
	if err != nil {
		if newValue == "" {
			return ""
		}
		return fmt.Sprintf("Изменено поле 'Отдел': ID на %s", newValue)
	}

	if name := r.otdelName(id); name != "" {
		return fmt.Sprintf("Заявка переведена в отдел: «%s»", name)
	}

	return fmt.Sprintf("Изменено поле 'Отдел': ID на %s", newValue)
}

func (r *historyReferenceResolver) structureChangeLine(comment string) string {
	return humanizeHistoryStructureComment(comment, r.structureChangeNameByField)
}

func humanizeHistoryStructureComment(comment string, lookup func(field string, id uint64) string) string {
	comment = strings.TrimSpace(comment)
	const prefix = "Смена структуры:"
	if comment == "" || !strings.HasPrefix(comment, prefix) {
		return comment
	}

	body := strings.TrimSpace(strings.TrimPrefix(comment, prefix))
	if body == "" {
		return comment
	}

	rawParts := strings.Split(body, ";")
	parts := make([]string, 0, len(rawParts))
	for _, rawPart := range rawParts {
		if part := humanizeHistoryStructurePart(rawPart, lookup); part != "" {
			parts = append(parts, part)
		}
	}

	if len(parts) == 0 {
		return comment
	}

	return prefix + " " + strings.Join(parts, "; ")
}

func humanizeHistoryStructurePart(part string, lookup func(field string, id uint64) string) string {
	part = strings.TrimSpace(part)
	if part == "" {
		return ""
	}

	fieldAndValue := strings.SplitN(part, ":", 2)
	if len(fieldAndValue) != 2 {
		return part
	}

	field := strings.TrimSpace(fieldAndValue[0])
	label, ok := historyStructureFieldLabel(field)
	if !ok {
		return part
	}

	transition := strings.TrimSpace(fieldAndValue[1])
	values := splitHistoryStructureTransition(transition)
	if len(values) != 2 {
		value, humanized := resolveHistoryStructureValue(field, transition, lookup)
		if !humanized {
			return part
		}
		return fmt.Sprintf("%s: %s", label, value)
	}

	oldValue, oldHumanized := resolveHistoryStructureValue(field, values[0], lookup)
	newValue, newHumanized := resolveHistoryStructureValue(field, values[1], lookup)

	if !oldHumanized && !newHumanized {
		return part
	}

	switch {
	case oldValue != "" && newValue != "":
		return fmt.Sprintf("%s: %s → %s", label, oldValue, newValue)
	case newValue != "":
		return fmt.Sprintf("%s: %s", label, newValue)
	case oldValue != "":
		return fmt.Sprintf("%s: %s → —", label, oldValue)
	default:
		return label
	}
}

func splitHistoryStructureTransition(raw string) []string {
	for _, separator := range []string{"→", "->", "в†’"} {
		if strings.Contains(raw, separator) {
			return strings.SplitN(raw, separator, 2)
		}
	}
	return []string{raw}
}

func resolveHistoryStructureValue(field, raw string, lookup func(field string, id uint64) string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "—" {
		return "", true
	}

	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return raw, true
	}

	if lookup != nil {
		if name := strings.TrimSpace(lookup(field, id)); name != "" {
			return fmt.Sprintf("«%s»", name), true
		}
	}

	return raw, true
}

func historyStructureFieldLabel(field string) (string, bool) {
	switch field {
	case "department_id":
		return "департамент", true
	case "otdel_id":
		return "отдел", true
	case "branch_id":
		return "филиал", true
	case "office_id":
		return "офис", true
	default:
		return "", false
	}
}

func (r *historyReferenceResolver) structureChangeNameByField(field string, id uint64) string {
	switch field {
	case "department_id":
		return r.departmentName(id)
	case "otdel_id":
		return r.otdelName(id)
	case "branch_id":
		return r.branchName(id)
	case "office_id":
		return r.officeName(id)
	default:
		return ""
	}
}

func (r *historyReferenceResolver) statusChangeLine(event repositories.OrderHistoryItem, newValue string) string {
	if name := strings.TrimSpace(utils.NullStringToString(event.NewStatusName)); name != "" {
		return fmt.Sprintf("Установлен статус: «%s»", name)
	}

	id, err := strconv.ParseUint(newValue, 10, 64)
	if err != nil {
		if newValue == "" {
			return ""
		}
		return fmt.Sprintf("Установлен статус: ID %s", newValue)
	}

	if name := r.statusName(id); name != "" {
		return fmt.Sprintf("Установлен статус: «%s»", name)
	}

	return fmt.Sprintf("Установлен статус: ID %s", newValue)
}

func (r *historyReferenceResolver) priorityChangeLine(newValue string) string {
	id, err := strconv.ParseUint(newValue, 10, 64)
	if err != nil {
		if newValue == "" {
			return ""
		}
		return fmt.Sprintf("Установлен приоритет: ID %s", newValue)
	}

	if name := r.priorityName(id); name != "" {
		return fmt.Sprintf("Установлен приоритет: «%s»", name)
	}

	return fmt.Sprintf("Установлен приоритет: ID %s", newValue)
}

func (r *historyReferenceResolver) departmentName(id uint64) string {
	if r.departmentSeen[id] {
		return r.departmentNames[id]
	}
	r.departmentSeen[id] = true

	dept, err := r.service.departmentRepo.FindDepartment(r.ctx, id)
	if err != nil {
		r.service.logger.Warn("failed to resolve department for order history", zap.Uint64("departmentID", id), zap.Error(err))
		return ""
	}

	r.departmentNames[id] = dept.Name
	return dept.Name
}

func (r *historyReferenceResolver) otdelName(id uint64) string {
	if r.otdelSeen[id] {
		return r.otdelNames[id]
	}
	r.otdelSeen[id] = true

	otdel, err := r.service.otdelRepo.FindOtdel(r.ctx, id)
	if err != nil {
		r.service.logger.Warn("failed to resolve otdel for order history", zap.Uint64("otdelID", id), zap.Error(err))
		return ""
	}

	r.otdelNames[id] = otdel.Name
	return otdel.Name
}

func (r *historyReferenceResolver) branchName(id uint64) string {
	if r.branchSeen[id] {
		return r.branchNames[id]
	}
	r.branchSeen[id] = true

	if r.service.branchRepo == nil {
		return ""
	}

	branch, err := r.service.branchRepo.FindBranch(r.ctx, id)
	if err != nil {
		r.service.logger.Warn("failed to resolve branch for order history", zap.Uint64("branchID", id), zap.Error(err))
		return ""
	}

	r.branchNames[id] = branch.Name
	return branch.Name
}

func (r *historyReferenceResolver) officeName(id uint64) string {
	if r.officeSeen[id] {
		return r.officeNames[id]
	}
	r.officeSeen[id] = true

	if r.service.officeRepo == nil {
		return ""
	}

	office, err := r.service.officeRepo.FindOffice(r.ctx, id)
	if err != nil {
		r.service.logger.Warn("failed to resolve office for order history", zap.Uint64("officeID", id), zap.Error(err))
		return ""
	}

	r.officeNames[id] = office.Name
	return office.Name
}

func (r *historyReferenceResolver) statusName(id uint64) string {
	if r.statusSeen[id] {
		return r.statusNames[id]
	}
	r.statusSeen[id] = true

	status, err := r.service.statusRepo.FindStatus(r.ctx, id)
	if err != nil {
		r.service.logger.Warn("failed to resolve status for order history", zap.Uint64("statusID", id), zap.Error(err))
		return ""
	}

	r.statusNames[id] = status.Name
	return status.Name
}

func (r *historyReferenceResolver) priorityName(id uint64) string {
	if r.prioritySeen[id] {
		return r.priorityNames[id]
	}
	r.prioritySeen[id] = true

	priority, err := r.service.priorityRepo.FindByID(r.ctx, id)
	if err != nil {
		r.service.logger.Warn("failed to resolve priority for order history", zap.Uint64("priorityID", id), zap.Error(err))
		return ""
	}

	r.priorityNames[id] = priority.Name
	return priority.Name
}
