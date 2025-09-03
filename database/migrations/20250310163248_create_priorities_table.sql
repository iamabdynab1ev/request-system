-- +goose Up
-- +goose StatementBegin
CREATE TABLE priorities (
    id BIGSERIAL PRIMARY KEY,
    icon_small VARCHAR(100) NOT NULL,
    icon_big VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    rate BIGINT NOT NULL,
    code VARCHAR(50) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

INSERT INTO priorities (name, rate, icon_small, icon_big, code) VALUES
('Критический', 100, 'icon_critical_16.png', 'icon_critical_24.png', 'CRITICAL'),
('Высокий', 75, 'icon_high_16.png', 'icon_high_24.png', 'HIGH'),
('Средний', 50, 'icon_medium_16.png', 'icon_medium_24.png', 'MEDIUM'),
('Низкий', 25, 'icon_low_16.png', 'icon_low_24.png', 'LOW');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS priorities;
-- +goose StatementEnd
