-- Minimal applications table so the list endpoint has a source.
-- #6 (create application) extends it with its own migration.

CREATE TABLE applications (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  app_key VARCHAR(40) NOT NULL,
  name VARCHAR(200) NOT NULL,
  description TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_applications_key (app_key)
) ENGINE=InnoDB;
