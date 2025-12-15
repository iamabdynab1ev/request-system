package repositories

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
)

const (
	ruleTable = "order_routing_rules"
	// ВАЖНО: Список полей должен совпадать со структурой базы данных
	// и порядком сканирования в методе scanRow
	ruleFields = "id, rule_name, order_type_id, department_id, otdel_id, branch_id, office_id, assign_to_position_id, status_id, created_at, updated_at"
)

type OrderRoutingRuleRepositoryInterface interface {
	Create(ctx context.Context, tx pgx.Tx, rule *entities.OrderRoutingRule) (int, error)
	Update(ctx context.Context, tx pgx.Tx, rule *entities.OrderRoutingRule) error
	Delete(ctx context.Context, tx pgx.Tx, id int) error
	FindByID(ctx context.Context, id int) (*entities.OrderRoutingRule, error)
	GetAll(ctx context.Context, limit, offset uint64, search string) ([]*entities.OrderRoutingRule, uint64, error)
	FindByTypeID(ctx context.Context, tx pgx.Tx, orderTypeID int) (*entities.OrderRoutingRule, error)
	ExistsByOrderTypeID(ctx context.Context, tx pgx.Tx, orderTypeID int) (bool, error)
}

type orderRoutingRuleRepository struct {
	storage *pgxpool.Pool
}

func NewOrderRoutingRuleRepository(storage *pgxpool.Pool) OrderRoutingRuleRepositoryInterface {
	return &orderRoutingRuleRepository{storage: storage}
}

// scanRow маппит строку из БД в структуру Go. Порядок полей критически важен!
func (r *orderRoutingRuleRepository) scanRow(row pgx.Row) (*entities.OrderRoutingRule, error) {
	var rule entities.OrderRoutingRule
	err := row.Scan(
		&rule.ID,
		&rule.RuleName,
		&rule.OrderTypeID,
		&rule.DepartmentID,
		&rule.OtdelID,
		&rule.BranchID,   // Новое поле
		&rule.OfficeID,   // Новое поле
		&rule.PositionID, // В БД это assign_to_position_id
		&rule.StatusID,
		&rule.CreatedAt, // BaseEntity поле
		&rule.UpdatedAt, // BaseEntity поле
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка сканирования правила маршрутизации: %w", err)
	}
	return &rule, nil
}

func (r *orderRoutingRuleRepository) Create(ctx context.Context, tx pgx.Tx, rule *entities.OrderRoutingRule) (int, error) {
	// Добавляем branch_id и office_id в INSERT
	query := `INSERT INTO order_routing_rules 
		(rule_name, order_type_id, department_id, otdel_id, branch_id, office_id, assign_to_position_id, status_id) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) 
		RETURNING id`

	var id int
	err := tx.QueryRow(ctx, query,
		rule.RuleName,
		rule.OrderTypeID,
		rule.DepartmentID,
		rule.OtdelID,
		rule.BranchID, // !
		rule.OfficeID, // !
		rule.PositionID,
		rule.StatusID,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // нарушение уникальности
			return 0, apperrors.ErrConflict
		}
		return 0, fmt.Errorf("ошибка создания правила: %w", err)
	}
	return id, nil
}

func (r *orderRoutingRuleRepository) Update(ctx context.Context, tx pgx.Tx, rule *entities.OrderRoutingRule) error {
	query := `UPDATE order_routing_rules SET 
		rule_name = $1, 
		order_type_id = $2, 
		department_id = $3, 
		otdel_id = $4,              
		branch_id = $5,
		office_id = $6,
		assign_to_position_id = $7, 
		status_id = $8, 
		updated_at = NOW() 
		WHERE id = $9`

	res, err := tx.Exec(ctx, query,
		rule.RuleName,
		rule.OrderTypeID,
		rule.DepartmentID,
		rule.OtdelID,
		rule.BranchID,
		rule.OfficeID,
		rule.PositionID,
		rule.StatusID,
		rule.ID,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *orderRoutingRuleRepository) Delete(ctx context.Context, tx pgx.Tx, id int) error {
	query := "DELETE FROM order_routing_rules WHERE id = $1"
	res, err := tx.Exec(ctx, query, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // FK violation
			return apperrors.NewHttpError(http.StatusBadRequest, "Правило используется в заявках и не может быть удалено", err, nil)
		}
		return err
	}
	if res.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *orderRoutingRuleRepository) FindByID(ctx context.Context, id int) (*entities.OrderRoutingRule, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", ruleFields, ruleTable)
	row := r.storage.QueryRow(ctx, query, id)
	return r.scanRow(row)
}

func (r *orderRoutingRuleRepository) FindByTypeID(ctx context.Context, tx pgx.Tx, orderTypeID int) (*entities.OrderRoutingRule, error) {
	// StatusID = 2 берем как хардкод активного статуса.
	// Либо замени на константу constants.StatusActiveID, если она числовая (например 10)
	query := fmt.Sprintf(`
		SELECT %s FROM %s
		WHERE order_type_id = $1
		ORDER BY id DESC
		LIMIT 1;
	`, ruleFields, ruleTable)

	var row pgx.Row
	// Универсальная поддержка работы внутри транзакции
	if tx != nil {
		row = tx.QueryRow(ctx, query, orderTypeID)
	} else {
		row = r.storage.QueryRow(ctx, query, orderTypeID)
	}

	return r.scanRow(row)
}

func (r *orderRoutingRuleRepository) ExistsByOrderTypeID(ctx context.Context, tx pgx.Tx, orderTypeID int) (bool, error) {
	query := "SELECT EXISTS (SELECT 1 FROM order_routing_rules WHERE order_type_id = $1)"
	var exists bool
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, query, orderTypeID).Scan(&exists)
	} else {
		err = r.storage.QueryRow(ctx, query, orderTypeID).Scan(&exists)
	}
	if err != nil {
		return false, fmt.Errorf("ошибка проверки существования правила: %w", err)
	}
	return exists, nil
}

func (r *orderRoutingRuleRepository) GetAll(ctx context.Context, limit, offset uint64, search string) ([]*entities.OrderRoutingRule, uint64, error) {
	var total uint64

	// Считаем общее количество с параметризованным запросом
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", ruleTable)
	var countArgs []interface{}

	if search != "" {
		countQuery += " WHERE rule_name ILIKE $1"
		countArgs = append(countArgs, "%"+search+"%")
	}

	if err := r.storage.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Выбираем данные с параметризованным запросом
	query := fmt.Sprintf("SELECT %s FROM %s", ruleFields, ruleTable)
	var queryArgs []interface{}
	argIndex := 1

	if search != "" {
		query += fmt.Sprintf(" WHERE rule_name ILIKE $%d", argIndex)
		queryArgs = append(queryArgs, "%"+search+"%")
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY id DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := r.storage.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	rules := make([]*entities.OrderRoutingRule, 0, limit)
	for rows.Next() {
		rule, err := r.scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		rules = append(rules, rule)
	}

	return rules, total, rows.Err()
}
