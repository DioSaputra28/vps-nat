CREATE TABLE packages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    description text,
    cpu integer NOT NULL CHECK (cpu > 0),
    ram_mb integer NOT NULL CHECK (ram_mb > 0),
    disk_gb integer NOT NULL CHECK (disk_gb > 0),
    price bigint NOT NULL CHECK (price >= 0),
    duration_days integer NOT NULL CHECK (duration_days > 0),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
