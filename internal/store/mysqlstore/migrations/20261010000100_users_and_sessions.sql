-- Users, login identities, sessions, and the first-start marker.
-- All timestamps are UTC (the connection runs with time_zone='+00:00').

CREATE TABLE system_state (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  bootstrapped_at DATETIME(6) NULL
) ENGINE=InnoDB;

INSERT INTO system_state (id, bootstrapped_at) VALUES (1, NULL);

CREATE TABLE users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(32) NOT NULL,
  display_name VARCHAR(200) NOT NULL,
  email VARCHAR(320) NOT NULL DEFAULT '',
  phone VARCHAR(64) NOT NULL DEFAULT '',
  department VARCHAR(200) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL,
  is_admin BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_users_username (username)
) ENGINE=InnoDB;

CREATE TABLE login_identities (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  kind VARCHAR(16) NOT NULL,
  login_name VARCHAR(255) NOT NULL,
  password_hash VARCHAR(255) NULL,
  bound_at DATETIME(6) NOT NULL,
  bound_via VARCHAR(16) NOT NULL,
  UNIQUE KEY uq_identity_login (kind, login_name),
  UNIQUE KEY uq_identity_user_kind (user_id, kind),
  CONSTRAINT fk_identity_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB;

CREATE TABLE sessions (
  token_hash CHAR(64) NOT NULL PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  KEY ix_sessions_user (user_id),
  CONSTRAINT fk_session_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB;
