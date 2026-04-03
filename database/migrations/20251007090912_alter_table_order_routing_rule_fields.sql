-- +goose Up
-- +goose StatementBegin
SELECT 'up: applying unique constraints';

ALTER TABLE public.order_routing_rules
ADD CONSTRAINT unique_order_type_id_in_rules UNIQUE (order_type_id);


ALTER TABLE public.order_types
ADD CONSTRAINT order_types_name_unique UNIQUE (name);

ALTER TABLE public.order_types
ALTER COLUMN code SET NOT NULL;
ALTER TABLE public.order_types
ADD CONSTRAINT order_types_code_unique UNIQUE (code);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: reverting unique constraints';


ALTER TABLE public.order_types DROP CONSTRAINT IF EXISTS order_types_code_unique;
ALTER TABLE public.order_types ALTER COLUMN code DROP NOT NULL;
ALTER TABLE public.order_types DROP CONSTRAINT IF EXISTS order_types_name_unique;


ALTER TABLE public.order_routing_rules
DROP CONSTRAINT IF EXISTS unique_order_type_id_in_rules;

-- +goose StatementEnd