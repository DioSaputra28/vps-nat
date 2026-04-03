CREATE TABLE provisioning_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id uuid REFERENCES services(id) ON DELETE RESTRICT,
    order_id uuid REFERENCES orders(id) ON DELETE RESTRICT,
    job_type text NOT NULL CHECK (job_type IN ('provision', 'reinstall', 'change_password', 'reassign_port', 'upgrade', 'start', 'stop', 'restart', 'cancel')),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'success', 'failed', 'rolled_back')),
    incus_operation_id text,
    requested_by_type text NOT NULL CHECK (requested_by_type IN ('system', 'user', 'admin')),
    requested_by_id uuid,
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    error_message text,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (order_id IS NOT NULL OR service_id IS NOT NULL),
    CHECK (
        (requested_by_type = 'system' AND requested_by_id IS NULL)
        OR
        (requested_by_type IN ('user', 'admin') AND requested_by_id IS NOT NULL)
    )
);

CREATE INDEX provisioning_jobs_status_created_at_idx
    ON provisioning_jobs (status, created_at DESC);
