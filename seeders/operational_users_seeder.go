// Файл: seeders/operational_users_seeder.go

package seeders

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"request-system/internal/entities"
	"request-system/pkg/utils"
)

// Структура для удобного описания тестового пользователя
type TestUser struct {
	Fio          string
	Email        string
	Phone        string
	PositionCode string // Будем искать должность по КОДУ
	Description  string // Краткое описание для логов
}

// seedOperationalUsers создает оргструктуру Операционного Департамента и наполняет ее пользователями.
func seedOperationalUsers(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Создание оргструктуры и пользователей Операционного Департамента...")

	// --- ШАГ 1: Получаем ID всех необходимых должностей ---
	positionsMap, err := getPositionsMapByCode(ctx, db)
	if err != nil {
		return err
	}

	// --- ШАГ 2: Получаем ID статуса "Активен" и базового филиала ---
	activeStatusID, err := getStatusIDByCode(ctx, db, "ACTIVE")
	if err != nil {
		return err
	}

	branchID := uint64(1) // Предполагаем, что "Головной офис" имеет ID=1

	// --- ШАГ 3: Создаем Департамент ---
	opDeptID, err := findOrCreateDepartment(ctx, db, "Операционный Департамент", activeStatusID)
	if err != nil {
		return err
	}

	// --- ШАГ 4: Создаем Отделы внутри Департамента ---
	creditDeptID, err := findOrCreateOtdel(ctx, db, "Отдел принятия кредитных решений", opDeptID, activeStatusID)
	if err != nil {
		return err
	}

	monitoringDeptID, err := findOrCreateOtdel(ctx, db, "Отдел мониторинга и взыскания", opDeptID, activeStatusID)
	if err != nil {
		return err
	}

	okoDeptID, err := findOrCreateOtdel(ctx, db, "Отдел Операционно-кассового обслуживания", opDeptID, activeStatusID)
	if err != nil {
		return err
	}

	terminalDeptID, err := findOrCreateOtdel(ctx, db, "Отдел по управлению терминальной сети самообслуживания", opDeptID, activeStatusID)
	if err != nil {
		return err
	}

	// --- ШАГ 5: Создаем ВСЕХ Пользователей для каждого подразделения ---

	log.Println("    -> Создание руководства департамента...")
	createUser(ctx, db, TestUser{"Иванов Иван Иванович", "ivanov.i@test.tj", "992927770001", "DEPARTMENT_HEAD", "Директор Опер. департамента"}, opDeptID, nil, branchID, positionsMap, activeStatusID, true)
	createUser(ctx, db, TestUser{"Петров Петр Петрович", "petrov.p@test.tj", "992927770002", "DEPARTMENT_VICE_HEAD", "Зам. Директора Опер. департамента"}, opDeptID, nil, branchID, positionsMap, activeStatusID, false)

	log.Println("    -> Создание сотрудников Отдела принятия кредитных решений...")
	createUser(ctx, db, TestUser{"Сидоров Сидор Сидорович", "sidorov.s@test.tj", "992927770003", "OTDEL_HEAD", "Менеджер отдела"}, opDeptID, &creditDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Абдуллоев Абдулло", "abdulloev.a@test.tj", "992927770004", "LEADING_SPECIALIST", "Андеррайтер по МСБ"}, opDeptID, &creditDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Борисов Борис Борисович", "borisov.b@test.tj", "992927770005", "SPECIALIST", "Специалист отдела"}, opDeptID, &creditDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Назаров Назар Назарович", "nazarov.n@test.tj", "992927770015", "SPECIALIST", "Андеррайтер"}, opDeptID, &creditDeptID, branchID, positionsMap, activeStatusID, false)

	log.Println("    -> Создание сотрудников Отдела мониторинга и взыскания...")
	createUser(ctx, db, TestUser{"Васильев Василий Васильевич", "vasilyev.v@test.tj", "992927770006", "OTDEL_HEAD", "Менеджер отдела"}, opDeptID, &monitoringDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Григорьев Григорий Григорьевич", "grigoryev.g@test.tj", "992927770007", "LEADING_SPECIALIST", "Юрист по претензионной работе"}, opDeptID, &monitoringDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Давлатов Давлат Давлатович", "davlatov.d@test.tj", "992927770008", "SPECIALIST", "Региональный юрист"}, opDeptID, &monitoringDeptID, branchID, positionsMap, activeStatusID, false)

	log.Println("    -> Создание сотрудников Отдела Операционно-кассового обслуживания...")
	createUser(ctx, db, TestUser{"Егоров Егор Егорович", "egorov.e@test.tj", "992927770009", "OTDEL_HEAD", "Менеджер отдела ОКО"}, opDeptID, &okoDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Жуков Жук Жукович", "zhukov.z@test.tj", "992927770010", "SPECIALIST", "Специалист ОКО"}, opDeptID, &okoDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Зайцев Заяц Зайцевич", "zaytsev.z@test.tj", "992927770011", "SPECIALIST", "Специалист ОКО"}, opDeptID, &okoDeptID, branchID, positionsMap, activeStatusID, false)

	log.Println("    -> Создание сотрудников Отдела по управлению терминальной сети...")
	createUser(ctx, db, TestUser{"Кириллов Кирилл Кириллович", "kirillov.k@test.tj", "992927770012", "OTDEL_HEAD", "Менеджер отдела ТСС"}, opDeptID, &terminalDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Лебедев Лебедь Лебедевич", "lebedev.l@test.tj", "992927770013", "SPECIALIST", "Специалист ТСС"}, opDeptID, &terminalDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Михайлов Михаил Михайлович", "mikhaylov.m@test.tj", "992927770014", "SPECIALIST", "Специалист ТСС"}, opDeptID, &terminalDeptID, branchID, positionsMap, activeStatusID, false)

	log.Println("  - ✅ Оргструктура и пользователи Операционного Департамента успешно созданы.")
	return nil
}

// --- Вспомогательные функции (остаются без изменений) ---

func getPositionsMapByCode(ctx context.Context, db *pgxpool.Pool) (map[string]uint64, error) {
	rows, err := db.Query(ctx, "SELECT id, code FROM positions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positionsMap := make(map[string]uint64)
	for rows.Next() {
		var id uint64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			return nil, err
		}
		positionsMap[code] = id
	}
	return positionsMap, nil
}

func getStatusIDByCode(ctx context.Context, db *pgxpool.Pool, code string) (uint64, error) {
	var id uint64
	err := db.QueryRow(ctx, "SELECT id FROM statuses WHERE code = $1", code).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("статус с кодом '%s' не найден: %w", code, err)
	}
	return id, nil
}

func findOrCreateDepartment(ctx context.Context, db *pgxpool.Pool, name string, statusID uint64) (uint64, error) {
	var id uint64
	err := db.QueryRow(ctx, "SELECT id FROM departments WHERE name = $1", name).Scan(&id)
	if err == nil {
		return id, nil // Уже существует
	}

	err = db.QueryRow(ctx, "INSERT INTO departments (name, status_id) VALUES ($1, $2) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id", name, statusID).Scan(&id)
	return id, err
}

func findOrCreateOtdel(ctx context.Context, db *pgxpool.Pool, name string, departmentID, statusID uint64) (uint64, error) {
	var id uint64
	err := db.QueryRow(ctx, "SELECT id FROM otdels WHERE name = $1 AND department_id = $2", name, departmentID).Scan(&id)
	if err == nil {
		return id, nil // Уже существует
	}

	err = db.QueryRow(ctx, "INSERT INTO otdels (name, department_id, status_id) VALUES ($1, $2, $3) ON CONFLICT (name, department_id) DO UPDATE SET name = EXCLUDED.name RETURNING id", name, departmentID, statusID).Scan(&id)
	return id, err
}

func createUser(ctx context.Context, db *pgxpool.Pool, u TestUser, deptID uint64, otdelID *uint64, branchID uint64, positions map[string]uint64, statusID uint64, isHead bool) {
	// Проверяем, существует ли пользователь
	var existingID uint64
	err := db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", u.Email).Scan(&existingID)
	if err == nil {
		log.Printf("    - Пользователь %s (%s) уже существует, пропускаем.", u.Fio, u.Description)
		return
	}

	positionID, ok := positions[u.PositionCode]
	if !ok {
		log.Printf("ПРЕДУПРЕЖДЕНИЕ: Должность с кодом '%s' для пользователя %s не найдена! Пропускаем.", u.PositionCode, u.Fio)
		return
	}

	hashedPassword, _ := utils.HashPassword(u.Phone)
	mustChangePassword := true

	userEntity := &entities.User{
		Fio: u.Fio, Email: u.Email, PhoneNumber: u.Phone, Password: hashedPassword,
		StatusID: statusID, BranchID: branchID, DepartmentID: deptID, OtdelID: otdelID,
		PositionID: positionID, IsHead: &isHead, MustChangePassword: mustChangePassword,
	}

	query := `
		INSERT INTO users (fio, email, phone_number, password, status_id, branch_id, 
						   department_id, otdel_id, position_id, is_head, must_change_password) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = db.Exec(ctx, query,
		userEntity.Fio, userEntity.Email, userEntity.PhoneNumber, userEntity.Password,
		userEntity.StatusID, userEntity.BranchID, userEntity.DepartmentID, userEntity.OtdelID,
		userEntity.PositionID, userEntity.IsHead, userEntity.MustChangePassword,
	)
	if err != nil {
		log.Printf("ОШИБКА при создании пользователя %s: %v", u.Fio, err)
	} else {
		log.Printf("    - Успешно создан пользователь: %s (%s)", u.Fio, u.Description)
	}
}
