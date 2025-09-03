-- +goose Up
-- +goose StatementBegin
CREATE TABLE statuses (
    id SERIAL PRIMARY KEY,
    icon_small VARCHAR(100) NOT NULL,
    icon_big VARCHAR(100) NOT NULL,
    name VARCHAR(50) NOT NULL,
    type INT NOT NULL,
    code VARCHAR(50) UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO statuses (name, type, icon_small, icon_big, code) VALUES
('Открыто', 1, 'icon_open_16.png', 'icon_open_24.png', 'OPEN'),
('Закрыто', 2, 'icon_closed_16.png', 'icon_closed_24.png', 'CLOSED'),
('Выполнено', 3, 'icon_completed_16.png', 'icon_completed_24.png', 'COMPLETED'),
('В работе', 4, 'icon_in_progress_16.png', 'icon_in_progress_24.png', 'IN_PROGRESS'),
('Отклонить', 5, 'icon_rejected_16.png', 'icon_rejected_24.png', 'REJECTED'),
('Уточнение', 6, 'icon_clarification_16.png', 'icon_clarification_24.png', 'CLARIFICATION'),
('Доработка', 7, 'icon_refinement_16.png', 'icon_refinement_24.png', 'REFINEMENT'),
('Подтверждён', 8, 'icon_confirmed_16.png', 'icon_confirmed_24.png', 'CONFIRMED'),
('Сервис', 9, 'icon_service_16.png', 'icon_service_24.png', 'SERVICE');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS statuses;
-- +goose StatementEnd
