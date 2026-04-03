ALTER TABLE orders
    ADD CONSTRAINT orders_target_service_id_fkey
    FOREIGN KEY (target_service_id) REFERENCES services(id) ON DELETE RESTRICT;
