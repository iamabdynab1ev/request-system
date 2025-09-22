package seeders

import "request-system/internal/entities"

var permissionsData = []struct {
	Name        string
	Description string
}{
	// --- Общие права ---
	{Name: "superuser", Description: "Суперпользователь (полный доступ)"},

	// --- Заявки ---
	{Name: "order:create", Description: "Создание заявок"},
	{Name: "order:view", Description: "Просмотр заявок"},
	{Name: "order:update", Description: "Обновление заявок"},
	{Name: "order:delete", Description: "Удаление заявок"},
	{Name: "order:delegate", Description: "Делегирование (назначение исполнителя) заявок"},
	{Name: "order:reopen", Description: "Повторное открытие закрытых заявок"},

	// --- Пользователи ---
	{Name: "user:create", Description: "Создание пользователей"},
	{Name: "user:view", Description: "Просмотр пользователей"},
	{Name: "user:update", Description: "Обновление пользователей"},
	{Name: "user:delete", Description: "Удаление пользователей"},
	{Name: "user:password:reset", Description: "Сброс паролей пользователей"},

	// --- Профиль пользователя ---
	{Name: "profile:update", Description: "Обновление своего профиля"},
	{Name: "password:update", Description: "Обновление своего пароля"},

	// --- Роли и Права ---
	{Name: "role:create", Description: "Создание ролей"},
	{Name: "role:view", Description: "Просмотр ролей"},
	{Name: "role:update", Description: "Обновление ролей"},
	{Name: "role:delete", Description: "Удаление ролей"},
	{Name: "permission:view", Description: "Просмотр списка всех прав"},

	// --- Справочники ---
	{Name: "status:create", Description: "Создание статусов"},
	{Name: "status:view", Description: "Просмотр статусов"},
	{Name: "status:update", Description: "Обновление статусов"},
	{Name: "status:delete", Description: "Удаление статусов"},
	{Name: "priority:create", Description: "Создание приоритетов"},
	{Name: "priority:view", Description: "Просмотр приоритетов"},
	{Name: "priority:update", Description: "Обновление приоритетов"},
	{Name: "priority:delete", Description: "Удаление приоритетов"},
	{Name: "department:create", Description: "Создание департаментов"},
	{Name: "department:view", Description: "Просмотр департаментов"},
	{Name: "department:update", Description: "Обновление департаментов"},
	{Name: "department:delete", Description: "Удаление департаментов"},
	{Name: "otdel:create", Description: "Создание отделов"},
	{Name: "otdel:view", Description: "Просмотр отделов"},
	{Name: "otdel:update", Description: "Обновление отделов"},
	{Name: "otdel:delete", Description: "Удаление отделов"},
	{Name: "branch:create", Description: "Создание филиалов"},
	{Name: "branch:view", Description: "Просмотр филиалов"},
	{Name: "branch:update", Description: "Обновление филиалов"},
	{Name: "branch:delete", Description: "Удаление филиалов"},
	{Name: "office:create", Description: "Создание офисов"},
	{Name: "office:view", Description: "Просмотр офисов"},
	{Name: "office:update", Description: "Обновление офисов"},
	{Name: "office:delete", Description: "Удаление офисов"},
	{Name: "equipment:create", Description: "Создание оборудования"},
	{Name: "equipment:view", Description: "Просмотр оборудования"},
	{Name: "equipment:update", Description: "Обновление оборудования"},
	{Name: "equipment:delete", Description: "Удаление оборудования"},
	{Name: "equipment_type:create", Description: "Создание типов оборудования"},
	{Name: "equipment_type:view", Description: "Просмотр типов оборудования"},
	{Name: "equipment_type:update", Description: "Обновление типов оборудования"},
	{Name: "equipment_type:delete", Description: "Удаление типов оборудования"},
	{Name: "position:create", Description: "Создание должностей"},
	{Name: "position:view", Description: "Просмотр должностей"},
	{Name: "position:update", Description: "Обновление должностей"},
	{Name: "position:delete", Description: "Удаление должностей"},

	// --- Область видимости (Scopes) ---
	{Name: "scope:own", Description: "Область: Только свои данные"},
	{Name: "scope:department", Description: "Область: Данные своего департамента"},
	{Name: "scope:all", Description: "Область: Все данные (в рамках прав)"},
}

var statusesData = []struct {
	Name string
	Type int
	Code string
}{
	{Name: "Отклонено", Type: 1, Code: "REJECTED"},
	{Name: "Активный", Type: 2, Code: "ACTIVE"},
	{Name: "Неактивный", Type: 2, Code: "INACTIVE"},
	{Name: "Выполнено", Type: 1, Code: "COMPLETED"},
	{Name: "Открыто", Type: 3, Code: "OPEN"},
	{Name: "Закрыто", Type: 3, Code: "CLOSED"},
	{Name: "Доработка", Type: 3, Code: "REFINEMENT"},
	{Name: "В работе", Type: 1, Code: "IN_PROGRESS"},
	{Name: "Уточнение", Type: 1, Code: "CLARIFICATION"},
	{Name: "Сервис", Type: 1, Code: "SERVICE"},
	{Name: "Подтвержден", Type: 1, Code: "CONFIRMED"},
}

var prioritiesData = []struct {
	Name string
	Rate int
	Code string
}{
	{Name: "Критический", Rate: 1, Code: "CRITICAL"},
	{Name: "Высокий", Rate: 2, Code: "HIGH"},
	{Name: "Средний", Rate: 3, Code: "MEDIUM"},
	{Name: "Низкий", Rate: 4, Code: "LOW"},
}

var rolesData = []entities.Role{
	{Name: "Super Admin", Description: "Администратор системы"},
	{Name: "User", Description: "Пользователь"},
	{Name: "Viewing audit", Description: "Ревизор просмотра"},
	{Name: "Executor", Description: "Исполнитель"},
	{Name: "Admin", Description: "Администратор"},
}
