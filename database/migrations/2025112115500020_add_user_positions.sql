-- +goose Up
-- +goose StatementBegin

-- 1. Создаем таблицу связей M2M
CREATE TABLE IF NOT EXISTS public.user_positions (
    user_id BIGINT NOT NULL,
    position_id BIGINT NOT NULL,
    CONSTRAINT fk_user_positions_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_positions_position FOREIGN KEY (position_id) REFERENCES public.positions(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, position_id)
);

CREATE INDEX IF NOT EXISTS idx_user_positions_pos_id ON public.user_positions(position_id);


INSERT INTO public.user_positions (user_id, position_id)
SELECT id, position_id FROM public.users 
WHERE position_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS public.user_positions;
-- +goose StatementEnd

