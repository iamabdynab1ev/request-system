package seeders

import (
	"request-system/internal/entities"
	"request-system/pkg/constants"
)

var permissionsData = []struct {
	Name, Description string
}{
	{"scope:own", "Доступ к своим данным"},
	{"scope:all", "Глобальный доступ на изменение всех данных"},
	{"scope:all_view", "Глобальный доступ только на просмотр всех данных"},
	{"order:create", "Создание заявки"},
	{"order:view", "Просмотр заявки"},
	{"order:update", "Редактирование 'своей' заявки"},
	{"order:delete", "Удаление заявки"},
	{"user:create", "Создание пользователя"},
	{"user:view", "Просмотр пользователя"},
	{"user:update", "Обновление пользователя"},
	{"user:delete", "Удаление пользователя"},
	{"user:password:reset", "Сброс пароля пользователя"},
	{"profile:update", "Обновление своего профиля"},
	{"password:update", "Обновление своего пароля"},
	{"role:create", "Создание роли"},
	{"role:view", "Просмотр роли"},
	{"role:update", "Обновление роли"},
	{"role:delete", "Удаление роли"},
	{"permission:view", "Просмотр системной привилегии"},
	{"status:view", "Просмотр статуса"},
	{"priority:view", "Просмотр приоритета"},
	{"department:view", "Просмотр департамента"},
	{"otdel:view", "Просмотр отдела"},
	{"branch:view", "Просмотр филиала"},
	{"office:view", "Просмотр офиса"},
	{"equipment:view", "Просмотр оборудования"},
	{"equipment_type:view", "Просмотр типа оборудования"},
	{"order_type:view", "Просмотр типа заявки"},
	{"position:create", "Создание должности"},
	{"position:view", "Просмотр должности"},
	{"position:update", "Обновление должности"},
	{"position:delete", "Удаление должности"},
	{"order_rule:view", "Просмотр правила маршрутизации"},
	{"report:view", "Просмотр отчета"},
}

var statusesData = []struct {
	Name, Code string
	Type       int
}{
	{"Активный", "ACTIVE", 2},
	{"Неактивный", "INACTIVE", 2},
	{"Открыто", "OPEN", 3},
	{"В работе", "IN_PROGRESS", 1},
	{"Закрыто", "CLOSED", 3},
}

var prioritiesData = []struct {
	Name, Code string
	Rate       int
}{
	{"Критический", "CRITICAL", 1},
	{"Высокий", "HIGH", 2},
	{"Средний", "MEDIUM", 3},
	{"Низкий", "LOW", 4},
}

var rolesData = []entities.Role{
	{Name: "Admin", Description: "Администратор системы (управление доступом)"},
	{Name: "Developer", Description: "Разработчик системы (полный доступ)"},
	{Name: "User", Description: "Пользователь (создание заявок)"},
	{Name: "Executor", Description: "Исполнитель заявок"},
	{Name: "Viewing audit", Description: "Ревизор (только просмотр)"},
}

var positionsData = []struct {
	Name     string
	Type     constants.PositionType
	StatusID int
}{
	{"Администратор ИБ", constants.PositionTypeSpecialist, 2},
	{"Разработчик", constants.PositionTypeSpecialist, 2},
	{"Руководитель департамента", constants.PositionTypeHeadOfDepartment, 2},
	{"Руководитель отдела", constants.PositionTypeHeadOfOtdel, 2},
	{"Специалист", constants.PositionTypeSpecialist, 2},
}

var ordertypesData = []struct {
	Name, Code string
	StatusID   int
}{
	{"Запрос на оборудование", "EQUIPMENT", 1},
	{"Административный запрос", "ADMINISTRATIVE", 1},
}
