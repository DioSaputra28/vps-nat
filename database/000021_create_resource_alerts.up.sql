CREATE TABLE resource_alerts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id uuid REFERENCES services(id) ON DELETE RESTRICT,
    node_id uuid REFERENCES nodes(id) ON DELETE RESTRICT,
    alert_type text NOT NULL CHECK (alert_type IN ('cpu_high', 'ram_high')),
    threshold_percent integer NOT NULL CHECK (threshold_percent BETWEEN 1 AND 100),
    duration_minutes integer NOT NULL CHECK (duration_minutes > 0),
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    opened_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CHECK (service_id IS NOT NULL OR node_id IS NOT NULL)
);

CREATE INDEX resource_alerts_status_opened_at_idx
    ON resource_alerts (status, opened_at DESC);
