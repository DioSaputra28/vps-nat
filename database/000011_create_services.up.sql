CREATE TABLE services (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid NOT NULL UNIQUE REFERENCES orders(id) ON DELETE RESTRICT,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    current_package_id uuid NOT NULL REFERENCES packages(id) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'provisioning', 'active', 'stopped', 'suspended', 'failed', 'terminated')),
    billing_cycle_days integer NOT NULL CHECK (billing_cycle_days > 0),
    package_name_snapshot text NOT NULL,
    cpu_snapshot integer NOT NULL CHECK (cpu_snapshot > 0),
    ram_mb_snapshot integer NOT NULL CHECK (ram_mb_snapshot > 0),
    disk_gb_snapshot integer NOT NULL CHECK (disk_gb_snapshot > 0),
    price_snapshot bigint NOT NULL CHECK (price_snapshot >= 0),
    started_at timestamptz,
    expires_at timestamptz,
    canceled_at timestamptz,
    suspended_at timestamptz,
    terminated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX services_owner_user_id_status_expires_at_idx
    ON services (owner_user_id, status, expires_at);
