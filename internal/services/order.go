// Файл: internal/services/order_service.go
// ИСПРАВЛЕНИЕ: Теперь вызывается функция, работающая внутри транзакции

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type OrderServiceInterface interface {
	CreateOrder(ctx context.Context, data string, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	DelegateOrder(ctx context.Context, orderID uint64, dto dto.DelegateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
}

type OrderService struct {
	txManager    repositories.TxManagerInterface
	orderRepo    repositories.OrderRepositoryInterface
	userRepo     repositories.UserRepositoryInterface
	statusRepo   repositories.StatusRepositoryInterface
	priorityRepo repositories.PriorityRepositoryInterface
	attachRepo   repositories.AttachmentRepositoryInterface
	historyRepo  repositories.OrderHistoryRepositoryInterface
	fileStorage  filestorage.FileStorageInterface
	logger       *zap.Logger
}

func NewOrderService(
	txManager repositories.TxManagerInterface,
	orderRepo repositories.OrderRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface,
	attachRepo repositories.AttachmentRepositoryInterface,
	historyRepo repositories.OrderHistoryRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) OrderServiceInterface {
	return &OrderService{
		txManager: txManager, orderRepo: orderRepo, userRepo: userRepo,
		statusRepo: statusRepo, priorityRepo: priorityRepo, attachRepo: attachRepo,
		historyRepo: historyRepo, fileStorage: fileStorage, logger: logger,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, data string, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	creatorID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	var createDTO dto.CreateOrderDTO
	if err = json.Unmarshal([]byte(data), &createDTO); err != nil {
		return nil, apperrors.ErrBadRequest
	}

	var finalOrderID uint64

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		status, err := s.statusRepo.FindByCode(ctx, "OPEN")
		if err != nil {
			return fmt.Errorf("не найден статус по умолчанию 'OPEN': %w", err)
		}
		priority, err := s.priorityRepo.FindByCode(ctx, "MEDIUM")
		if err != nil {
			return fmt.Errorf("не найден приоритет по умолчанию 'MEDIUM': %w", err)
		}

		executor, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, createDTO.DepartmentID)
		if err != nil {
			return err
		}

		orderEntity := &entities.Order{
			Name:         createDTO.Name,
			Address:      createDTO.Address,
			DepartmentID: createDTO.DepartmentID,
			StatusID:     uint64(status.ID),
			PriorityID:   uint64(priority.ID),
			CreatorID:    uint64(creatorID),
			ExecutorID:   uint64(executor.ID),
		}

		orderID, err := s.orderRepo.Create(ctx, tx, orderEntity)
		if err != nil {
			return fmt.Errorf("не удалось создать запись о заявке в БД: %w", err)
		}

		finalOrderID = orderID
		createHistory := &entities.OrderHistory{OrderID: orderID, UserID: uint64(creatorID), EventType: "CREATE", Comment: &orderEntity.Name}
		if err := s.historyRepo.CreateInTx(ctx, tx, createHistory, nil); err != nil {
			s.logger.Error("[OrderService] ОШИБКА: Не удалось создать запись в истории (CREATE)")
			return err
		}

		delegationComment := fmt.Sprintf("Назначен ответственный: %s", executor.Fio)
		delegateHistory := &entities.OrderHistory{OrderID: orderID, UserID: uint64(creatorID), EventType: "DELEGATION", NewValue: &executor.Fio, Comment: &delegationComment}
		if err := s.historyRepo.CreateInTx(ctx, tx, delegateHistory, nil); err != nil {
			s.logger.Error("[OrderService] ОШИБКА: Не удалось создать запись в истории (DELEGATION)")
			return err
		}
		// --- БЛОК ДЛЯ СОХРАНЕНИЯ КОММЕНТАРИЯ В ИСТОРИИ ---
		if createDTO.Comment != nil && *createDTO.Comment != "" {
			commentHistory := &entities.OrderHistory{
				OrderID:   orderID,
				UserID:    uint64(creatorID),
				EventType: "COMMENT",
				Comment:   createDTO.Comment,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, commentHistory, nil); err != nil {
				s.logger.Error("[OrderService] ОШИБКА: Не удалось создать запись в истории (COMMENT)")
				return err
			}
		}
		if file != nil {
			s.logger.Info("[OrderService] УСЛОВИЕ 'if file != nil' ВЫПОЛНИЛОСЬ! Начинаю сохранение файла.")
			filePath, err := s.fileStorage.Save(file)
			if err != nil {
				s.logger.Error("[OrderService] Ошибка при сохранении файла на диск", zap.Error(err))
				return fmt.Errorf("не удалось сохранить файл: %w", err)
			}
			s.logger.Info("[OrderService] Файл успешно сохранен на диск", zap.String("path", filePath))

			attachEntity := &entities.Attachment{
				OrderID: orderID, UserID: uint64(creatorID), FileName: file.Filename,
				FilePath: filePath, FileType: file.Header.Get("Content-Type"), FileSize: file.Size,
			}

			_, err = s.attachRepo.Create(ctx, tx, attachEntity)
			if err != nil {
				s.logger.Error("[OrderService] Ошибка при сохранении информации о файле в БД", zap.Error(err))

				return fmt.Errorf("не удалось создать запись о вложении: %w", err)
			}
			s.logger.Info("[OrderService] Информация о файле успешно сохранена в БД")
			attachHistory := &entities.OrderHistory{
				OrderID:   orderID,
				UserID:    uint64(creatorID),
				EventType: "ATTACHMENT_ADDED",
				Comment:   &file.Filename,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, attachHistory, nil); err != nil {
				s.logger.Error("[OrderService] ОШИБКА: Не удалось создать запись в истории (ATTACHMENT_ADDED)")
				return err
			}

		} else {
			s.logger.Warn("[OrderService] УСЛОВИЕ 'if file != nil' НЕ ВЫПОЛНИЛОСЬ! Переменная file оказалась nil.")
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("ошибка при создании заявки. Исходная проблема: [%v]", err)
	}

	return s.buildOrderResponse(ctx, finalOrderID)
}
func (s *OrderService) DelegateOrder(ctx context.Context, orderID uint64, dto dto.DelegateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	actorID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		order, err := s.orderRepo.FindByID(ctx, orderID)
		if err != nil {
			return err
		}

		// Здесь ваша логика проверки прав доступа, можно будет добавить позже
		if order.ExecutorID != uint64(actorID) {
			// TODO: Add role check for Admin
		}

		hasChanges := false

		// ... (все if'ы для Name, Address, и т.д. остаются без изменений)
		
		if dto.ExecutorID != nil && order.ExecutorID != uint64(*dto.ExecutorID) {
			newExecutor, err := s.userRepo.FindUserByID(ctx, uint64(*dto.ExecutorID))
			if err != nil {
				s.logger.Warn("delegateOrder: attempt to assign non-existent executor", zap.Error(err), zap.Uint64p("executorID", dto.ExecutorID))
				return apperrors.ErrUserNotFound
			}

            // ---------- НАША НОВАЯ ПРОВЕРКА! ----------
            // Убеждаемся, что новый исполнитель работает в том же департаменте, что и заявка
            if newExecutor.DepartmentID != order.DepartmentID {
                s.logger.Warn("delegateOrder: Попытка назначить исполнителя из другого департамента",
                    zap.Uint64("orderID", orderID),
                    zap.Uint64("orderDepartmentID", order.DepartmentID),
                    zap.Uint64("executorDepartmentID", newExecutor.DepartmentID),
                )
                return apperrors.NewHttpError(400, "Нельзя назначить исполнителя из другого департамента.", nil)
            }
            // ---------------------------------------------

			history := &entities.OrderHistory{OrderID: orderID, UserID: uint64(actorID), EventType: "DELEGATION", NewValue: &newExecutor.Fio}
			if err := s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return err
			}

			order.ExecutorID = uint64(*dto.ExecutorID)
			hasChanges = true
		}

		// ... (все остальные if'ы для StatusID, PriorityID, Comment, File и т.д. остаются без изменений)

		if hasChanges {
			if err := s.orderRepo.Update(ctx, tx, order); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		s.logger.Error("ошибка при выполнении транзакции делегирования", zap.Error(err), zap.Uint64("orderID", orderID))
		return nil, err
	}
	return s.buildOrderResponse(ctx, orderID)
}

func (s *OrderService) buildOrderResponse(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error) {
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	creator, _ := s.userRepo.FindUserByID(ctx, uint64(order.CreatorID))
	executor, _ := s.userRepo.FindUserByID(ctx, order.ExecutorID)
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, orderID, 5, 0)

	creatorDTO := dto.ShortUserDTO{ID: order.CreatorID}
	if creator != nil {
		creatorDTO.Fio = creator.Fio
	}

	executorDTO := dto.ShortUserDTO{ID: uint64(order.ExecutorID)}
	if executor != nil {
		executorDTO.Fio = executor.Fio
	}

	var attachmentsDTO []dto.AttachmentResponseDTO
	for _, att := range attachments {
		attachmentsDTO = append(attachmentsDTO, dto.AttachmentResponseDTO{
			ID: att.ID, FileName: att.FileName, FileSize: att.FileSize,
			FileType: att.FileType, URL: "/static/" + att.FilePath,
		})
	}

	return &dto.OrderResponseDTO{
		ID:           order.ID,
		Name:         order.Name,
		Address:      order.Address,
		Creator:      creatorDTO,
		Executor:     executorDTO,
		DepartmentID: order.DepartmentID,
		StatusID:     order.StatusID,
		PriorityID:   order.PriorityID,
		Attachments:  attachmentsDTO,
		CreatedAt:    order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    order.UpdatedAt.Format(time.RFC3339),
	}, nil
}
