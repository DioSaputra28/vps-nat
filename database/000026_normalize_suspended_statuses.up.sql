UPDATE services
SET status = 'stopped',
    suspended_at = NULL
WHERE status = 'suspended';

UPDATE service_instances
SET status = 'stopped'
WHERE status = 'suspended';
