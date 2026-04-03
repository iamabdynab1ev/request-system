-- +goose Up
-- +goose StatementBegin
SELECT 'up: switching all INTEGER primary/foreign keys to BIGINT';

-- ШАГ 1: безопасное удаление всех внешних ключей
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN (SELECT conname, conrelid::regclass AS table_name 
              FROM pg_constraint 
              WHERE contype = 'f') 
    LOOP
        EXECUTE 'ALTER TABLE ' || r.table_name || ' DROP CONSTRAINT ' || r.conname;
    END LOOP;
END$$;

-- ШАГ 2: меняем типы всех INTEGER ID на BIGINT
ALTER TABLE public.branches ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.branches ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.departments ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.departments ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.equipment_types ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.equipments ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.equipments ALTER COLUMN branch_id SET DATA TYPE BIGINT;
ALTER TABLE public.equipments ALTER COLUMN office_id SET DATA TYPE BIGINT;
ALTER TABLE public.equipments ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.equipments ALTER COLUMN equipment_type_id SET DATA TYPE BIGINT;
ALTER TABLE public.offices ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.offices ALTER COLUMN branch_id SET DATA TYPE BIGINT;
ALTER TABLE public.offices ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_comments ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.order_comments ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_comments ALTER COLUMN order_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_comments ALTER COLUMN user_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_delegations ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.order_delegations ALTER COLUMN delegation_user_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_delegations ALTER COLUMN delegated_user_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_delegations ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_delegations ALTER COLUMN order_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_documents ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.order_documents ALTER COLUMN order_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_routing_rules ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.order_routing_rules ALTER COLUMN order_type_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_routing_rules ALTER COLUMN department_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_routing_rules ALTER COLUMN branch_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_routing_rules ALTER COLUMN office_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_routing_rules ALTER COLUMN otdel_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_routing_rules ALTER COLUMN assign_to_position_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_routing_rules ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.order_types ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.order_types ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN department_id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN otdel_id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN priority_id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN branch_id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN office_id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN equipment_id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN user_id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN executor_id SET DATA TYPE BIGINT;
ALTER TABLE public.orders ALTER COLUMN order_type_id SET DATA TYPE BIGINT;
ALTER TABLE public.otdels ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.otdels ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.otdels ALTER COLUMN department_id SET DATA TYPE BIGINT;
ALTER TABLE public.permissions ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.positions ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.positions ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.roles ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.roles ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.statuses ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.statuses ALTER COLUMN type SET DATA TYPE BIGINT;
ALTER TABLE public.users ALTER COLUMN id SET DATA TYPE BIGINT;
ALTER TABLE public.users ALTER COLUMN status_id SET DATA TYPE BIGINT;
ALTER TABLE public.users ALTER COLUMN branch_id SET DATA TYPE BIGINT;
ALTER TABLE public.users ALTER COLUMN office_id SET DATA TYPE BIGINT;
ALTER TABLE public.users ALTER COLUMN department_id SET DATA TYPE BIGINT;
ALTER TABLE public.users ALTER COLUMN otdel_id SET DATA TYPE BIGINT;
ALTER TABLE public.users ALTER COLUMN position_id SET DATA TYPE BIGINT;

-- ШАГ 3: воссоздаём все внешние ключи
ALTER TABLE ONLY public.attachments ADD CONSTRAINT fk_attachments_order FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.attachments ADD CONSTRAINT fk_attachments_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;
ALTER TABLE ONLY public.users ADD CONSTRAINT fk_branches_id FOREIGN KEY (branch_id) REFERENCES public.branches(id);
ALTER TABLE ONLY public.branches ADD CONSTRAINT fk_branches_status_id FOREIGN KEY (status_id) REFERENCES public.statuses(id) ON DELETE RESTRICT;
ALTER TABLE ONLY public.users ADD CONSTRAINT fk_departments_id FOREIGN KEY (department_id) REFERENCES public.departments(id);
ALTER TABLE ONLY public.equipments ADD CONSTRAINT fk_equipment_branch_id FOREIGN KEY (branch_id) REFERENCES public.branches(id);
ALTER TABLE ONLY public.equipments ADD CONSTRAINT fk_equipment_equipment_type_id FOREIGN KEY (equipment_type_id) REFERENCES public.equipment_types(id);
ALTER TABLE ONLY public.equipments ADD CONSTRAINT fk_equipment_office_id FOREIGN KEY (office_id) REFERENCES public.offices(id);
ALTER TABLE ONLY public.equipments ADD CONSTRAINT fk_equipment_status_id FOREIGN KEY (status_id) REFERENCES public.statuses(id);
ALTER TABLE ONLY public.offices ADD CONSTRAINT fk_office_branches_id FOREIGN KEY (branch_id) REFERENCES public.branches(id);
ALTER TABLE ONLY public.offices ADD CONSTRAINT fk_office_status_id FOREIGN KEY (status_id) REFERENCES public.statuses(id);
ALTER TABLE ONLY public.users ADD CONSTRAINT fk_offices_id FOREIGN KEY (office_id) REFERENCES public.offices(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.order_comments ADD CONSTRAINT fk_order_comments_order_id FOREIGN KEY (order_id) REFERENCES public.orders(id);
ALTER TABLE ONLY public.order_comments ADD CONSTRAINT fk_order_comments_status_id FOREIGN KEY (status_id) REFERENCES public.statuses(id);
ALTER TABLE ONLY public.order_comments ADD CONSTRAINT fk_order_comments_user_id FOREIGN KEY (user_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.order_delegations ADD CONSTRAINT fk_order_delegations_delegated_user_id FOREIGN KEY (delegated_user_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.order_delegations ADD CONSTRAINT fk_order_delegations_delegation_user_id FOREIGN KEY (delegation_user_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.order_delegations ADD CONSTRAINT fk_order_delegations_order_id FOREIGN KEY (order_id) REFERENCES public.orders(id);
ALTER TABLE ONLY public.order_delegations ADD CONSTRAINT fk_order_delegations_status_id FOREIGN KEY (status_id) REFERENCES public.statuses(id);
ALTER TABLE ONLY public.order_documents ADD CONSTRAINT fk_order_documents_order_id FOREIGN KEY (order_id) REFERENCES public.orders(id);
ALTER TABLE ONLY public.order_history ADD CONSTRAINT fk_order_history_attachment FOREIGN KEY (attachment_id) REFERENCES public.attachments(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.order_history ADD CONSTRAINT fk_order_history_order FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.order_history ADD CONSTRAINT fk_order_history_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;
ALTER TABLE ONLY public.orders ADD CONSTRAINT fk_orders_branch_id FOREIGN KEY (branch_id) REFERENCES public.branches(id);
ALTER TABLE ONLY public.orders ADD CONSTRAINT fk_orders_department_id FOREIGN KEY (department_id) REFERENCES public.departments(id);
ALTER TABLE ONLY public.orders ADD CONSTRAINT fk_orders_equipment_id FOREIGN KEY (equipment_id) REFERENCES public.equipments(id);
ALTER TABLE ONLY public.orders ADD CONSTRAINT fk_orders_executor_id FOREIGN KEY (executor_id) REFERENCES public.users(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.orders ADD CONSTRAINT fk_orders_office_id FOREIGN KEY (office_id) REFERENCES public.offices(id);
ALTER TABLE ONLY public.orders ADD CONSTRAINT fk_orders_order_type_id FOREIGN KEY (order_type_id) REFERENCES public.order_types(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.orders ADD CONSTRAINT fk_orders_otdel_id FOREIGN KEY (otdel_id) REFERENCES public.otdels(id);
ALTER TABLE ONLY public.orders ADD CONSTRAINT fk_orders_priority_id FOREIGN KEY (priority_id) REFERENCES public.priorities(id);
ALTER TABLE ONLY public.orders ADD CONSTRAINT fk_orders_status_id FOREIGN KEY (status_id) REFERENCES public.statuses(id);
ALTER TABLE ONLY public.orders ADD CONSTRAINT fk_orders_user_id FOREIGN KEY (user_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.otdels ADD CONSTRAINT fk_otdel_departments FOREIGN KEY (department_id) REFERENCES public.departments(id);
ALTER TABLE ONLY public.otdels ADD CONSTRAINT fk_otdel_status FOREIGN KEY (status_id) REFERENCES public.statuses(id);
ALTER TABLE ONLY public.users ADD CONSTRAINT fk_otdels_id FOREIGN KEY (otdel_id) REFERENCES public.otdels(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.users ADD CONSTRAINT fk_position_id FOREIGN KEY (position_id) REFERENCES public.positions(id);
ALTER TABLE ONLY public.role_permissions ADD CONSTRAINT fk_role_permissions_permission_id FOREIGN KEY (permission_id) REFERENCES public.permissions(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.role_permissions ADD CONSTRAINT fk_role_permissions_role_id FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.roles ADD CONSTRAINT fk_roles_status_id FOREIGN KEY (status_id) REFERENCES public.statuses(id);
ALTER TABLE ONLY public.departments ADD CONSTRAINT fk_status_id FOREIGN KEY (status_id) REFERENCES public.statuses(id);
ALTER TABLE ONLY public.users ADD CONSTRAINT fk_status_id FOREIGN KEY (status_id) REFERENCES public.statuses(id);
ALTER TABLE ONLY public.user_permission_denials ADD CONSTRAINT fk_user_permission_denials_permission_id FOREIGN KEY (permission_id) REFERENCES public.permissions(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.user_permission_denials ADD CONSTRAINT fk_user_permission_denials_user_id FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.user_permissions ADD CONSTRAINT fk_user_permissions_permission_id FOREIGN KEY (permission_id) REFERENCES public.permissions(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.user_permissions ADD CONSTRAINT fk_user_permissions_user_id FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.user_roles ADD CONSTRAINT fk_user_roles_role_id FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.user_roles ADD CONSTRAINT fk_user_roles_user_id FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.users ADD CONSTRAINT fk_users_position_id FOREIGN KEY (position_id) REFERENCES public.positions(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.order_routing_rules ADD CONSTRAINT order_routing_rules_assign_to_position_id_fkey FOREIGN KEY (assign_to_position_id) REFERENCES public.positions(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.order_routing_rules ADD CONSTRAINT order_routing_rules_order_type_id_fkey FOREIGN KEY (order_type_id) REFERENCES public.order_types(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.positions ADD CONSTRAINT positions_branch_id_fkey FOREIGN KEY (branch_id) REFERENCES public.branches(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.positions ADD CONSTRAINT positions_department_id_fkey FOREIGN KEY (department_id) REFERENCES public.departments(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.positions ADD CONSTRAINT positions_office_id_fkey FOREIGN KEY (office_id) REFERENCES public.offices(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.positions ADD CONSTRAINT positions_otdel_id_fkey FOREIGN KEY (otdel_id) REFERENCES public.otdels(id) ON DELETE SET NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: reverting BIGINT back to INTEGER (ATTENTION: data loss possible if IDs are > 2 billion)';
-- Здесь должна быть обратная логика:
-- 1. Удалить ключи
-- 2. ALTER COLUMN ... TYPE INTEGER
-- 3. Вернуть ключи
-- (Можно оставить этот блок пустым, если ты не планируешь делать откат)
-- +goose StatementEnd



