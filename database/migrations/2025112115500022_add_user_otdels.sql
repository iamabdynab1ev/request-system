-- +goose Up
-- +goose StatementBegin


CREATE TABLE IF NOT EXISTS public.user_otdels (
    user_id BIGINT NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    otdel_id BIGINT NOT NULL REFERENCES public.otdels(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, otdel_id)
);

INSERT INTO public.user_otdels (user_id, otdel_id)
SELECT id, otdel_id FROM public.users 
WHERE otdel_id IS NOT NULL 
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS public.user_otdels;


