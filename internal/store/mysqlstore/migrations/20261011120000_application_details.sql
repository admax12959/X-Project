-- Application details (#6): repository binding, deployment info, Owner/PE members,
-- the lines table (only the mainline for now; #7 adds maintenance lines) and the
-- append-only change history.
-- The mainline branch name lives only on the mainline row of app_lines.

ALTER TABLE applications
  ADD COLUMN platform_id BIGINT UNSIGNED NOT NULL,
  ADD COLUMN repo_path VARCHAR(512) NOT NULL,
  ADD COLUMN repo_project_id VARCHAR(64) NOT NULL,
  ADD COLUMN deploy_info TEXT NOT NULL,
  ADD CONSTRAINT fk_applications_platform FOREIGN KEY (platform_id) REFERENCES code_platforms (id);

-- LINES is a reserved word in MySQL, hence the prefix.
CREATE TABLE app_lines (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  application_id BIGINT UNSIGNED NOT NULL,
  is_main BOOLEAN NOT NULL,
  name VARCHAR(200) NOT NULL,
  branch VARCHAR(255) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  main_flag TINYINT GENERATED ALWAYS AS (IF(is_main, 1, NULL)) STORED,
  UNIQUE KEY uq_app_lines_branch (application_id, branch),
  UNIQUE KEY uq_app_lines_main (application_id, main_flag),
  CONSTRAINT fk_app_lines_app FOREIGN KEY (application_id) REFERENCES applications (id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE application_members (
  application_id BIGINT UNSIGNED NOT NULL,
  role VARCHAR(8) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  position SMALLINT UNSIGNED NOT NULL,
  PRIMARY KEY (application_id, role, user_id),
  KEY ix_members_user (user_id),
  CONSTRAINT fk_members_app FOREIGN KEY (application_id) REFERENCES applications (id) ON DELETE CASCADE,
  CONSTRAINT fk_members_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB;

-- Append-only: the repository port only appends and lists. User references inside
-- params are stored as IDs and resolved to names when read (docs/dev.md).
CREATE TABLE change_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  application_id BIGINT UNSIGNED NOT NULL,
  occurred_at DATETIME(6) NOT NULL,
  actor_user_id BIGINT UNSIGNED NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  params JSON NOT NULL,
  KEY ix_change_events_app (application_id, occurred_at, id),
  CONSTRAINT fk_change_events_app FOREIGN KEY (application_id) REFERENCES applications (id) ON DELETE CASCADE,
  CONSTRAINT fk_change_events_actor FOREIGN KEY (actor_user_id) REFERENCES users (id)
) ENGINE=InnoDB;
