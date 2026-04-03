CREATE TABLE server_costs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id uuid NOT NULL REFERENCES nodes(id) ON DELETE RESTRICT,
    purchase_cost bigint NOT NULL CHECK (purchase_cost >= 0),
    notes text,
    recorded_at timestamptz NOT NULL DEFAULT now(),
    created_by_admin_id uuid REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now()
);
