CREATE TABLE service_domains (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id uuid NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    domain text NOT NULL UNIQUE,
    target_port integer NOT NULL CHECK (target_port BETWEEN 1 AND 65535),
    proxy_mode text NOT NULL CHECK (proxy_mode IN ('http', 'https', 'http_and_https')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'failed', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX service_domains_service_id_status_idx
    ON service_domains (service_id, status);
