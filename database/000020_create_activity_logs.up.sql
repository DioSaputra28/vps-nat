CREATE TABLE activity_logs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_type text NOT NULL CHECK (actor_type IN ('system', 'user', 'admin')),
    actor_id uuid,
    action text NOT NULL,
    target_type text NOT NULL,
    target_id uuid,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (actor_type = 'system' AND actor_id IS NULL)
        OR
        (actor_type IN ('user', 'admin') AND actor_id IS NOT NULL)
    )
);

CREATE INDEX activity_logs_target_type_target_id_created_at_idx
    ON activity_logs (target_type, target_id, created_at DESC);
