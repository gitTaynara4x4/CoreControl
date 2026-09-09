-- CoreControl v10.40 - Gateway local para Wake-on-LAN sem depender do roteador
ALTER TABLE enrollment_tokens
    ADD COLUMN IF NOT EXISTS purpose VARCHAR(24) NOT NULL DEFAULT 'computer';

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS device_kind VARCHAR(24) NOT NULL DEFAULT 'computer';

CREATE INDEX IF NOT EXISTS ix_enrollment_tokens_purpose ON enrollment_tokens (purpose);
CREATE INDEX IF NOT EXISTS ix_devices_device_kind ON devices (device_kind);
