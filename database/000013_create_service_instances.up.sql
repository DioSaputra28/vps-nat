CREATE TABLE service_instances (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id uuid NOT NULL UNIQUE REFERENCES services(id) ON DELETE RESTRICT,
    node_id uuid NOT NULL REFERENCES nodes(id) ON DELETE RESTRICT,
    incus_instance_name text NOT NULL UNIQUE,
    image_alias text NOT NULL,
    os_family text,
    internal_ip inet,
    main_public_ip inet,
    ssh_port integer CHECK (ssh_port IS NULL OR (ssh_port BETWEEN 1 AND 65535)),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'provisioning', 'running', 'stopped', 'suspended', 'failed', 'deleted')),
    last_incus_operation_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
