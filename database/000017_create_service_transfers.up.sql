CREATE TABLE service_transfers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id uuid NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    from_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    to_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reason text,
    created_by_type text NOT NULL CHECK (created_by_type IN ('system', 'user', 'admin')),
    created_by_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (from_user_id <> to_user_id),
    CHECK (
        (created_by_type = 'system' AND created_by_id IS NULL)
        OR
        (created_by_type IN ('user', 'admin') AND created_by_id IS NOT NULL)
    )
);
