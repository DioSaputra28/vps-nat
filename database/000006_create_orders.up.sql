CREATE TABLE orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    package_id uuid NOT NULL REFERENCES packages(id) ON DELETE RESTRICT,
    target_service_id uuid,
    order_type text NOT NULL CHECK (order_type IN ('purchase', 'renewal', 'upgrade')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'awaiting_payment', 'paid', 'processing', 'completed', 'failed', 'canceled')),
    payment_status text NOT NULL DEFAULT 'pending' CHECK (payment_status IN ('pending', 'paid', 'failed', 'expired', 'refunded', 'canceled')),
    payment_method text CHECK (payment_method IN ('wallet', 'qris', 'manual')),
    selected_image_alias text,
    package_name_snapshot text NOT NULL,
    cpu_snapshot integer NOT NULL CHECK (cpu_snapshot > 0),
    ram_mb_snapshot integer NOT NULL CHECK (ram_mb_snapshot > 0),
    disk_gb_snapshot integer NOT NULL CHECK (disk_gb_snapshot > 0),
    price_snapshot bigint NOT NULL CHECK (price_snapshot >= 0),
    duration_days_snapshot integer NOT NULL CHECK (duration_days_snapshot > 0),
    total_amount bigint NOT NULL CHECK (total_amount >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    paid_at timestamptz,
    canceled_at timestamptz,
    failed_at timestamptz,
    CHECK (
        (order_type = 'purchase' AND target_service_id IS NULL)
        OR
        (order_type IN ('renewal', 'upgrade') AND target_service_id IS NOT NULL)
    )
);

CREATE INDEX orders_user_id_status_created_at_idx
    ON orders (user_id, status, created_at DESC);

CREATE INDEX orders_target_service_id_idx
    ON orders (target_service_id);
