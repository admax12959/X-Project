-- Recovery codes (#31, ADR-0006): one-time codes that stand in for the authenticator's code
-- when the phone is lost. Only the hash is kept; a code is spent by setting used_at, and
-- turning the second factor off or generating a new set removes the old rows.
CREATE TABLE user_recovery_codes (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  code_hash CHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  used_at DATETIME(6) NULL,
  KEY ix_recovery_user (user_id, used_at),
  CONSTRAINT fk_recovery_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB;
