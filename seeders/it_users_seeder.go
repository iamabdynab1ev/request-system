// Файл: seeders/it_users_seeder.go

package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedITUsers создает оргструктуру IT-Департамента и наполняет ее пользователями.
func seedITUsers(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Создание оргструктуры и пользователей IT-Департамента...")

	// --- Получаем все необходимые ID (должности, статус, филиал) ---
	positionsMap, err := getPositionsMapByCode(ctx, db)
	if err != nil {
		return err
	}
	activeStatusID, err := getStatusIDByCode(ctx, db, "ACTIVE")
	if err != nil {
		return err
	}
	branchID := uint64(1) // Головной офис

	// --- Создаем Департамент ---
	itDeptID, err := findOrCreateDepartment(ctx, db, "Департамент Информационных технологий", activeStatusID)
	if err != nil {
		return err
	}

	// --- Создаем Отделы ---
	adminDeptID, err := findOrCreateOtdel(ctx, db, "Отдел администрирования ИТ-инфраструктуры", itDeptID, activeStatusID)
	if err != nil {
		return err
	}
	devDeptID, err := findOrCreateOtdel(ctx, db, "Отдел цифровых разработок", itDeptID, activeStatusID)
	if err != nil {
		return err
	}
	supportDeptID, err := findOrCreateOtdel(ctx, db, "Отдел технической поддержки", itDeptID, activeStatusID)
	if err != nil {
		return err
	}
	isDeptID, err := findOrCreateOtdel(ctx, db, "Отдел Сопровождения информационных систем", itDeptID, activeStatusID)
	if err != nil {
		return err
	}
	analyticsDeptID, err := findOrCreateOtdel(ctx, db, "Отдел ИТ-аналитики", itDeptID, activeStatusID)
	if err != nil {
		return err
	}

	// --- Создаем Пользователей ---

	log.Println("    -> Создание руководства IT-департамента...")
	createUser(ctx, db, TestUser{"Рахмонов Рустам Эмомалиевич", "rakhmonov.r@test.tj", "992928880001", "DEPARTMENT_HEAD", "Директор департамента ИТ"}, itDeptID, nil, branchID, positionsMap, activeStatusID, true)
	createUser(ctx, db, TestUser{"Собиров Собир Собирович", "sobirov.s@test.tj", "992928880002", "DEPARTMENT_VICE_HEAD", "Заместитель Директора ИТ"}, itDeptID, nil, branchID, positionsMap, activeStatusID, false)

	log.Println("    -> Отдел администрирования ИТ-инфраструктуры...")
	createUser(ctx, db, TestUser{"Алиев Алишер Алиевич", "aliev.a@test.tj", "992928880003", "OTDEL_HEAD", "Менеджер"}, itDeptID, &adminDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Баротов Барот Баротович", "barotov.b@test.tj", "992928880004", "SENIOR_SPECIALIST", "Администратор БД (Синьор)"}, itDeptID, &adminDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Каримов Карим Каримович", "karimov.k@test.tj", "992928880005", "SENIOR_SPECIALIST", "DevOps-инженер (Синьор)"}, itDeptID, &adminDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Латипов Латип Латипович", "latipov.l@test.tj", "992928880006", "MIDDLE_SPECIALIST", "Инженер по ИТ-инфраструктуре (Мидл)"}, itDeptID, &adminDeptID, branchID, positionsMap, activeStatusID, false)

	log.Println("    -> Отдел цифровых разработок...")
	createUser(ctx, db, TestUser{"Махмудов Махмуд Махмудович", "makhmudov.m@test.tj", "992928880007", "OTDEL_HEAD", "Менеджер"}, itDeptID, &devDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Наимов Наим Наимович", "naimov.n@test.tj", "992928880008", "TEAM_LEAD", "Фронт-энд разработчик (Тимлид)"}, itDeptID, &devDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Олимов Олим Олимович", "olimov.o@test.tj", "992928880009", "TEAM_LEAD", "Бэк-энд разработчик (Тимлид)"}, itDeptID, &devDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Пулодов Пулод Пулодович", "pulodov.p@test.tj", "992928880010", "SENIOR_SPECIALIST", "Мобильный разработчик (Синьор)"}, itDeptID, &devDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Саидов Саид Саидович", "saidov.s@test.tj", "992928880011", "SENIOR_SPECIALIST", "Системный аналитик (Синьор)"}, itDeptID, &devDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Толибов Толиб Толибович", "tolibov.t@test.tj", "992928880012", "MIDDLE_SPECIALIST", "QA-инженер (Мидл)"}, itDeptID, &devDeptID, branchID, positionsMap, activeStatusID, false)

	log.Println("    -> Отдел технической поддержки...")
	createUser(ctx, db, TestUser{"Умаров Умар Умарович", "umarov.u@test.tj", "992928880013", "OTDEL_HEAD", "Менеджер"}, itDeptID, &supportDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Файзуллоев Файзулло", "fayzulloev.f@test.tj", "992928880014", "LEADING_SPECIALIST", "Специалист по поддержке (Главный)"}, itDeptID, &supportDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Хасанов Хасан Хасанович", "hasanov.h@test.tj", "992928880015", "ELECTRONICS_ENGINEER", "Инженер-электронщик (Старший)"}, itDeptID, &supportDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Шарипов Шариф Шарипович", "sharipov.s@test.tj", "992928880016", "TECHNICIAN", "Техник филиала (Ведущий)"}, itDeptID, &supportDeptID, branchID, positionsMap, activeStatusID, false)

	log.Println("    -> Отдел Сопровождения ИС...")
	createUser(ctx, db, TestUser{"Юсупов Юсуф Юсупович", "yusupov.y@test.tj", "992928880017", "OTDEL_HEAD", "Менеджер"}, itDeptID, &isDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Ятимов Ятим Ятимович", "yatimov.y@test.tj", "992928880018", "LEADING_SPECIALIST", "Специалист по сопровождению (Главный)"}, itDeptID, &isDeptID, branchID, positionsMap, activeStatusID, false)

	log.Println("    -> Отдел ИТ-аналитики...")
	createUser(ctx, db, TestUser{"Азизов Азиз Азизович", "azizov.a@test.tj", "992928880019", "OTDEL_HEAD", "Менеджер"}, itDeptID, &analyticsDeptID, branchID, positionsMap, activeStatusID, false)
	createUser(ctx, db, TestUser{"Бобоев Бобо Бобоевич", "boboev.b@test.tj", "992928880020", "TEAM_LEAD", "Data-аналитик (Тимлид)"}, itDeptID, &analyticsDeptID, branchID, positionsMap, activeStatusID, false)

	log.Println("  - ✅ Оргструктура и пользователи IT-Департамента успешно созданы.")
	return nil
}
