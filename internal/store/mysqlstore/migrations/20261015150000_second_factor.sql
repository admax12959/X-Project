-- Second factor (#22, ADR-0006): the authenticators of a user, the failed-attempt counter
-- that locks the second step of a login, and the short-lived tokens of a login whose first
-- factor passed.

-- One row per authenticator. The type leaves room for passkeys later: a passkey keeps its
-- credential in the same row (secret_enc) or in a table of its own, and a user may have
-- several of them, which is why (user_id, type) is not unique.
--   status:         'pending' = set up but the first code was not confirmed yet; 'enabled'.
--   secret_enc:     totp: the 160-bit secret sealed with the master key (nonce || ciphertext),
--                   bound to the user; key_version is the version of the master key.
--   last_used_step: totp: the time step of the last accepted code; a code of that step or an
--                   earlier one is a replay. 0 = none yet.
CREATE TABLE user_authenticators (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  type VARCHAR(16) NOT NULL,
  status VARCHAR(8) NOT NULL,
  secret_enc VARBINARY(512) NULL,
  key_version INT NOT NULL DEFAULT 0,
  last_used_step BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  enabled_at DATETIME(6) NULL,
  KEY ix_authenticators_user (user_id, type, status),
  CONSTRAINT fk_authenticator_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB;

-- Failed second-step attempts of a user, counted across logins; locked_until is set when the
-- count reaches the threshold and the count starts again.
CREATE TABLE second_factor_lockouts (
  user_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
  failures INT NOT NULL,
  locked_until DATETIME(6) NULL,
  CONSTRAINT fk_lockout_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB;

-- The short-lived token handed out when the first factor passed. It is not a session: it
-- only lets its holder submit the second step.
CREATE TABLE login_challenges (
  token_hash CHAR(64) NOT NULL PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  must_change_password BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME(6) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  KEY ix_challenges_expires (expires_at),
  CONSTRAINT fk_challenge_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB;
