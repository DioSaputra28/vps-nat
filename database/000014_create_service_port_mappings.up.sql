CREATE TABLE service_port_mappings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id uuid NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    mapping_type text NOT NULL CHECK (mapping_type IN ('ssh', 'custom', 'reverse_proxy')),
    public_ip inet NOT NULL,
    public_port integer NOT NULL CHECK (public_port BETWEEN 1 AND 65535),
    protocol text NOT NULL CHECK (protocol IN ('tcp', 'udp')),
    target_ip inet NOT NULL,
    target_port integer NOT NULL CHECK (target_port BETWEEN 1 AND 65535),
    description text,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (public_ip, public_port, protocol)
);

CREATE INDEX service_port_mappings_service_id_is_active_idx
    ON service_port_mappings (service_id, is_active);
