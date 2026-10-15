-- System configuration kept in the database (#19, ADR-0005): one row per section ("ldap",
-- later "smtp", "storage", ...) with the non-secret fields as JSON.
CREATE TABLE system_config (
  section VARCHAR(32) NOT NULL PRIMARY KEY,
  data JSON NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB;

-- The secret fields of a section (passwords, tokens, access keys), one row per field:
-- AES-256-GCM ciphertext (nonce || sealed) bound to section and field, plus the version of
-- the master key that sealed it. No API ever reads these back.
CREATE TABLE system_config_secrets (
  section VARCHAR(32) NOT NULL,
  field_name VARCHAR(64) NOT NULL,
  value_enc VARBINARY(4096) NOT NULL,
  key_version INT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (section, field_name)
) ENGINE=InnoDB;
