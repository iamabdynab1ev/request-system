package seeders

import "request-system/internal/entities"

var permissionsData = []struct {
	Name        string
	Description string
}{
	// --- ОБЛАСТИ ВИДИМОСТИ (SCOPES) ---
	{Name: "scope:own", Description: "Доступ к своим данным (где пользователь Участник)"},
	{Name: "scope:otdel", Description: "Доступ к данным в рамках своего Отдела"},
	{Name: "scope:office", Description: "Доступ к данным в рамках своего Офиса"},
	{Name: "scope:branch", Description: "Доступ к данным в рамках своего Филиала"},
	{Name: "scope:department", Description: "Доступ к данным в рамках своего Департамента"},
	{Name: "scope:all", Description: "Глобальный доступ на изменение всех данных"},
	{Name: "scope:all_view", Description: "Глобальный доступ только на просмотр всех данных"},

	// --- УСИЛИТЕЛИ РЕДАКТИРОВАНИЯ ЗАЯВОК ---
	{Name: "order:update_in_otdel_scope", Description: "Редактирование любой заявки в своем Отделе"},
	{Name: "order:update_in_office_scope", Description: "Редактирование любой заявки в своем Офисе"},
	{Name: "order:update_in_branch_scope", Description: "Редактирование любой заявки в своем Филиале"},
	{Name: "order:update_in_department_scope", Description: "Редактирование любой заявки в своем Департаменте"},

	// --- ОБЩИЕ ДЕЙСТВИЯ С ЗАЯВКАМИ ---
	{Name: "order:create", Description: "Создание заявки"},
	{Name: "order:view", Description: "Просмотр заявки"},
	{Name: "order:update", Description: "Редактирование 'своей' заявки"},
	{Name: "order:delete", Description: "Удаление заявки"},

	// --- ПОЛЯ ЗАЯВКИ ПРИ СОЗДАНИИ ---
	{Name: "order:create:name", Description: "Создание заявки: Поле 'Название'"},
	{Name: "order:create:address", Description: "Создание заявки: Поле 'Адрес'"},
	{Name: "order:create:department_id", Description: "Создание заявки: Поле 'Департамент'"},
	{Name: "order:create:otdel_id", Description: "Создание заявки: Поле 'Отдел'"},
	{Name: "order:create:branch_id", Description: "Создание заявки: Поле 'Филиал'"},
	{Name: "order:create:office_id", Description: "Создание заявки: Поле 'Офис'"},
	{Name: "order:create:equipment_id", Description: "Создание заявки: Поле 'Оборудование'"},
	{Name: "order:create:equipment_type_id", Description: "Создание заявки: Поле 'Тип оборудования'"},
	{Name: "order:create:executor_id", Description: "Создание заявки: Поле 'Исполнитель'"},
	{Name: "order:create:priority_id", Description: "Создание заявки: Поле 'Приоритет'"},
	{Name: "order:create:duration", Description: "Создание заявки: Поле 'Срок выполнения'"},
	{Name: "order:create:file", Description: "Создание заявки: Поле 'Файл'"},
	{Name: "order:create:comment", Description: "Создание заявки: Поле 'Комментарий'"},

	// --- ПОЛЯ ЗАЯВКИ ПРИ ОБНОВЛЕНИИ ---
	{Name: "order:update:name", Description: "Изменение поля 'Название'"},
	{Name: "order:update:address", Description: "Изменение поля 'Адрес'"},
	{Name: "order:update:department_id", Description: "Изменение поля 'Департамент'"},
	{Name: "order:update:otdel_id", Description: "Изменение поля 'Отдел'"},
	{Name: "order:update:branch_id", Description: "Изменение поля 'Филиал'"},
	{Name: "order:update:office_id", Description: "Изменение поля 'Офис'"},
	{Name: "order:update:equipment_id", Description: "Изменение поля 'Оборудование'"},
	{Name: "order:update:equipment_type_id", Description: "Изменение поля 'Тип оборудования'"},
	{Name: "order:update:executor_id", Description: "Изменение 'Исполнителя' (делегирование)"},
	{Name: "order:update:status_id", Description: "Изменение 'Статуса'"},
	{Name: "order:update:priority_id", Description: "Изменение 'Приоритета'"},
	{Name: "order:update:duration", Description: "Изменение 'Срока выполнения'"},
	{Name: "order:update:comment", Description: "Добавление 'Комментария'"},
	{Name: "order:update:file", Description: "Прикрепление файла"},
	{Name: "order:update:reopen", Description: "Переоткрытие закрытой заявки"},

	// --- ПОЛЬЗОВАТЕЛИ И ПРОФИЛЬ ---
	{Name: "user:create", Description: "Создание пользователя"},
	{Name: "user:view", Description: "Просмотр пользователя"},
	{Name: "user:update", Description: "Обновление пользователя"},
	{Name: "user:delete", Description: "Удаление пользователя"},
	{Name: "user:password:reset", Description: "Сброс пароля пользователя"},
	{Name: "profile:update", Description: "Обновление своего профиля"},
	{Name: "password:update", Description: "Обновление своего пароля"},

	// --- РОЛИ И ПРАВА ДОСТУПА ---
	{Name: "role:create", Description: "Создание роли"},
	{Name: "role:view", Description: "Просмотр роли"},
	{Name: "role:update", Description: "Обновление роли"},
	{Name: "role:delete", Description: "Удаление роли"},
	{Name: "permission:create", Description: "Создание системной привилегии"},
	{Name: "permission:update", Description: "Обновление системной привилегии"},
	{Name: "permission:delete", Description: "Удаление системной привилегии"},
	{Name: "permission:view", Description: "Просмотр системной привилегии"},

	// --- СПРАВОЧНИКИ ---
	{Name: "status:create", Description: "Создание статуса"},
	{Name: "status:view", Description: "Просмотр статуса"},
	{Name: "status:update", Description: "Обновление статуса"},
	{Name: "status:delete", Description: "Удаление статуса"},
	{Name: "priority:create", Description: "Создание приоритета"},
	{Name: "priority:view", Description: "Просмотр приоритета"},
	{Name: "priority:update", Description: "Обновление приоритета"},
	{Name: "priority:delete", Description: "Удаление приоритета"},
	{Name: "department:create", Description: "Создание департамента"},
	{Name: "department:view", Description: "Просмотр департамента"},
	{Name: "department:update", Description: "Обновление департамента"},
	{Name: "department:delete", Description: "Удаление департамента"},
	{Name: "otdel:create", Description: "Создание отдела"},
	{Name: "otdel:view", Description: "Просмотр отдела"},
	{Name: "otdel:update", Description: "Обновление отдела"},
	{Name: "otdel:delete", Description: "Удаление отдела"},
	{Name: "branch:create", Description: "Создание филиала"},
	{Name: "branch:view", Description: "Просмотр филиала"},
	{Name: "branch:update", Description: "Обновление филиала"},
	{Name: "branch:delete", Description: "Удаление филиала"},
	{Name: "office:create", Description: "Создание офиса"},
	{Name: "office:view", Description: "Просмотр офиса"},
	{Name: "office:update", Description: "Обновление офиса"},
	{Name: "office:delete", Description: "Удаление офиса"},
	{Name: "equipment:create", Description: "Создание оборудования"},
	{Name: "equipment:view", Description: "Просмотр оборудования"},
	{Name: "equipment:update", Description: "Обновление оборудования"},
	{Name: "equipment:delete", Description: "Удаление оборудования"},
	{Name: "equipment_type:create", Description: "Создание типа оборудования"},
	{Name: "equipment_type:view", Description: "Просмотр типа оборудования"},
	{Name: "equipment_type:update", Description: "Обновление типа оборудования"},
	{Name: "equipment_type:delete", Description: "Удаление типа оборудования"},

	// --- ТИПЫ ЗАЯВОК ---
	{Name: "order_type:create", Description: "Создание типа заявки"},
	{Name: "order_type:view", Description: "Просмотр типа заявки"},
	{Name: "order_type:update", Description: "Обновление типа заявки"},
	{Name: "order_type:delete", Description: "Удаление типа заявки"},

	// --- ДОЛЖНОСТИ ---
	{Name: "position:create", Description: "Создание должности"},
	{Name: "position:view", Description: "Просмотр должности"},
	{Name: "position:update", Description: "Обновление должности"},
	{Name: "position:delete", Description: "Удаление должности"},

	// --- ПРАВИЛА МАРШРУТИЗАЦИИ заявок ---
	{Name: "order_rule:create", Description: "Создание правила маршрутизации"},
	{Name: "order_rule:view", Description: "Просмотр правила маршрутизации"},
	{Name: "order_rule:update", Description: "Обновление правила маршрутизации"},
	{Name: "order_rule:delete", Description: "Удаление правила маршрутизации"},

	// --- ОТЧЕТЫ ---
	{Name: "report:view", Description: "Просмотр отчета"},
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
	{Name: "Admin", Description: "ИБ - Управление доступом"},
	{Name: "Developer", Description: "Разработчик системы"},
	{Name: "User", Description: "Заявитель (стандартный пользователь)"},
	{Name: "Head of department", Description: "Руководитель департамента"},
	{Name: "Executor", Description: "Исполнитель заявок"},
	{Name: "Viewing audit", Description: "Ревизор (только просмотр)"},
}

var positionsData = []struct {
	Name     string
	Code     string
	Level    int
	StatusID int
}{
	// --- ДОЛЖНОСТИ ---

	{Name: "Руководитель", Code: "HEAD", Level: 300, StatusID: 2},
	{Name: "Заместитель руководителя", Code: "VICE_HEAD", Level: 290, StatusID: 2},

	{Name: "Руководитель департамента", Code: "DEPARTMENT_HEAD", Level: 200, StatusID: 2},
	{Name: "Заместитель руководителя департамента", Code: "DEPARTMENT_VICE_HEAD", Level: 190, StatusID: 2},

	{Name: "Руководитель отдела", Code: "OTDEL_HEAD", Level: 100, StatusID: 2},
	{Name: "Заместитель руководителя отдела", Code: "OTDEL_VICE_HEAD", Level: 90, StatusID: 2},

	{Name: "Ведущий специалист", Code: "LEADING_SPECIALIST", Level: 80, StatusID: 2},
	{Name: "Специалист", Code: "SPECIALIST", Level: 70, StatusID: 2},
	{Name: "Младший специалист", Code: "JUNIOR_SPECIALIST", Level: 60, StatusID: 2},

	{Name: "Администратор ИБ", Code: "SECURITY_ADMIN", Level: 1000, StatusID: 2},
	{Name: "Разработчик", Code: "DEVELOPER", Level: 1000, StatusID: 2},
	{Name: "Системный администратор", Code: "SYSTEM_ADMIN", Level: 1000, StatusID: 2},
	{Name: "Тим лид", Code: "TEAM_LEAD", Level: 100, StatusID: 2},
	{Name: "Сенёр специалист", Code: "SENIOR_SPECIALIST", Level: 80, StatusID: 2},
	{Name: "Мидл специалист", Code: "MIDDLE_SPECIALIST", Level: 70, StatusID: 2},
}

var ordertypesData = []struct {
	Name     string
	Code     string
	StatusID int
}{
	{Name: "Оборудование", Code: "EQUIPMENT", StatusID: 1},
	{Name: "Административый", Code: "ADMINISTRATIVE", StatusID: 1},
}
