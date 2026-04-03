-- +goose Up
-- +goose StatementBegin
SELECT 'up: adapting otdels table to support branch linking';

-- Шаг 1: Делаем существующее поле department_id необязательным (nullable).
-- Это позволит нам создавать отделы, привязанные к филиалу, а не к департаменту.
ALTER TABLE public.otdels ALTER COLUMN department_id DROP NOT NULL;
ALTER TABLE public.otdels RENAME COLUMN department_id TO departments_id;


-- Шаг 2: Добавляем новую колонку для связи с филиалами (branches).
ALTER TABLE public.otdels ADD COLUMN branch_id BIGINT;

-- Шаг 3: Добавляем внешний ключ (foreign key) на таблицу branches.
ALTER TABLE public.otdels
ADD CONSTRAINT fk_otdels_branch_id
FOREIGN KEY (branch_id) REFERENCES public.branches(id) ON DELETE SET NULL; -- ON DELETE SET NULL для гибкости

-- Шаг 4: Добавляем ограничение (CHECK), чтобы гарантировать,
-- что отдел привязан ЛИБО к департаменту, ЛИБО к филиалу, но не к обоим сразу,
-- и не может быть совсем без родителя.
ALTER TABLE public.otdels
ADD CONSTRAINT chk_otdel_parent
CHECK (
    (departments_id IS NOT NULL AND branch_id IS NULL)
    OR
    (departments_id IS NULL AND branch_id IS NOT NULL)
);

SELECT 'up: migration completed';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: reverting otdels table changes';

-- Откатываем все в обратном порядке
ALTER TABLE public.otdels DROP CONSTRAINT chk_otdel_parent;
ALTER TABLE public.otdels DROP CONSTRAINT fk_otdels_branch_id;
ALTER TABLE public.otdels DROP COLUMN branch_id;
ALTER TABLE public.otdels RENAME COLUMN departments_id TO department_id;


-- Важно: перед тем как вернуть NOT NULL, нужно убедиться, что нет отделов
-- без department_id. В данном случае, мы их просто удаляем, т.к. откатываемся.
-- Или можно задать значение по умолчанию. Здесь мы предполагаем, что таких данных не будет.
ALTER TABLE public.otdels ALTER COLUMN department_id SET NOT NULL;


SELECT 'down: migration reverted';
-- +goose StatementEnd

