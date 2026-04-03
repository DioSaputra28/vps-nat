CREATE TABLE service_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id uuid NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    event_type text NOT NULL,
    actor_type text NOT NULL CHECK (actor_type IN ('system', 'user', 'admin')),
    actor_id uuid,
    summary text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (actor_type = 'system' AND actor_id IS NULL)
        OR
        (actor_type IN ('user', 'admin') AND actor_id IS NOT NULL)
    )
);
