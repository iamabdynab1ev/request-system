-- +goose Up
-- +goose StatementBegin
SELECT 'up: introducing hierarchical (self-referencing) structure for otdels and offices';


ALTER TABLE public.otdels DROP CONSTRAINT IF EXISTS chk_otdel_parent;

ALTER TABLE public.otdels ALTER COLUMN departments_id DROP NOT NULL;


ALTER TABLE public.otdels ADD COLUMN parent_id BIGINT;


ALTER TABLE public.otdels
ADD CONSTRAINT fk_otdels_parent_id
FOREIGN KEY (parent_id) REFERENCES public.otdels(id) ON DELETE SET NULL; 

ALTER TABLE public.otdels
ADD CONSTRAINT chk_otdel_single_parent
CHECK (
    (CASE WHEN parent_id IS NOT NULL THEN 1 ELSE 0 END) +
    (CASE WHEN departments_id IS NOT NULL THEN 1 ELSE 0 END) +
    (CASE WHEN branch_id IS NOT NULL THEN 1 ELSE 0 END) = 1
);


ALTER TABLE public.offices ALTER COLUMN branch_id DROP NOT NULL;


ALTER TABLE public.offices ADD COLUMN parent_id BIGINT;


ALTER TABLE public.offices
ADD CONSTRAINT fk_offices_parent_id
FOREIGN KEY (parent_id) REFERENCES public.offices(id) ON DELETE SET NULL;

ALTER TABLE public.offices
ADD CONSTRAINT chk_office_single_parent
CHECK (
    (parent_id IS NOT NULL AND branch_id IS NULL)
    OR
    (parent_id IS NULL AND branch_id IS NOT NULL)
);

SELECT 'up: migration to hierarchical structure completed successfully';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: reverting hierarchical structure for otdels and offices';

-- Откатываем изменения для OTDELS
ALTER TABLE public.otdels DROP CONSTRAINT chk_otdel_single_parent;
ALTER TABLE public.otdels DROP CONSTRAINT fk_otdels_parent_id;
ALTER TABLE public.otdels DROP COLUMN parent_id;

ALTER TABLE public.offices DROP CONSTRAINT chk_office_single_parent;
ALTER TABLE public.offices DROP CONSTRAINT fk_offices_parent_id;
ALTER TABLE public.offices DROP COLUMN parent_id;
ALTER TABLE public.offices ALTER COLUMN branch_id SET NOT NULL;


SELECT 'down: migration reverted';
-- +goose StatementEnd

