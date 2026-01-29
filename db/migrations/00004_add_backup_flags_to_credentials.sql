ALTER TABLE user_credentials ADD COLUMN backup_eligible BOOLEAN DEFAULT 0;
ALTER TABLE user_credentials ADD COLUMN backup_state BOOLEAN DEFAULT 0;
