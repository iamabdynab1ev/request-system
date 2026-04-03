-- +goose Up
-- +goose StatementBegin
SELECT 'up: redesigning positions table for hierarchy and performance';

ALTER TABLE public.positions DROP COLUMN IF EXISTS code;
ALTER TABLE public.positions DROP COLUMN IF EXISTS level;

ALTER TABLE public.positions ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id) ON DELETE SET NULL;
ALTER TABLE public.positions ADD COLUMN IF NOT EXISTS otdel_id BIGINT REFERENCES otdels(id) ON DELETE SET NULL;
ALTER TABLE public.positions ADD COLUMN IF NOT EXISTS branch_id BIGINT REFERENCES branches(id) ON DELETE SET NULL;
ALTER TABLE public.positions ADD COLUMN IF NOT EXISTS office_id BIGINT REFERENCES offices(id) ON DELETE SET NULL;
ALTER TABLE public.positions ADD COLUMN IF NOT EXISTS "type" VARCHAR(100);

CREATE INDEX IF NOT EXISTS idx_positions_type_org ON public.positions ("type", department_id, otdel_id);
CREATE INDEX IF NOT EXISTS idx_users_position_org ON public.users (position_id, department_id, otdel_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: reverting positions table redesign and indexes';

DROP INDEX IF EXISTS idx_users_position_org;
DROP INDEX IF EXISTS idx_positions_type_org;

ALTER TABLE public.positions DROP COLUMN IF EXISTS "type";
ALTER TABLE public.positions DROP COLUMN IF EXISTS office_id;
ALTER TABLE public.positions DROP COLUMN IF EXISTS branch_id;
ALTER TABLE public.positions DROP COLUMN IF EXISTS otdel_id;
ALTER TABLE public.positions DROP COLUMN IF EXISTS department_id;

ALTER TABLE public.positions ADD COLUMN IF NOT EXISTS level INT NOT NULL DEFAULT 0;
ALTER TABLE public.positions ADD COLUMN IF NOT EXISTS code VARCHAR(100);
CREATE UNIQUE INDEX IF NOT EXISTS positions_code_unique ON public.positions (code);

-- +goose StatementEnd

