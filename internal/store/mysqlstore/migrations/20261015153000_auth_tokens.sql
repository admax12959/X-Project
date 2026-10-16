-- One-time tokens (#21): the link in an invitation or password recovery mail carries one.
-- Only the SHA-256 of the token is stored (the token itself exists in the mail and nowhere
-- else), with what it is for, when it expires and whether it has been used. Later mail flows
-- (email verification, two-factor reset) add purposes, not tables.
CREATE TABLE auth_tokens (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  token_hash CHAR(64) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  purpose VARCHAR(32) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  used_at DATETIME(6) NULL,
  UNIQUE KEY uq_auth_tokens_hash (token_hash),
  KEY ix_auth_tokens_user (user_id),
  CONSTRAINT fk_auth_token_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB;

-- Events that count towards a rate limit (password recovery requests, per email and per IP).
-- bucket_key is a name plus a hash or an address, never a plain email address.
CREATE TABLE rate_events (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  bucket_key VARCHAR(255) NOT NULL,
  occurred_at DATETIME(6) NOT NULL,
  KEY ix_rate_events_bucket (bucket_key, occurred_at)
) ENGINE=InnoDB;
