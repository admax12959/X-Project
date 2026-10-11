-- Line lifecycle (#10): one row per status change of a maintenance line, with the time
-- and the actor. The line detail shows them in time order. from_status / to_status are
-- 'active', 'frozen' or 'retired'. Rows go with their line (and with the application).

CREATE TABLE app_line_status_changes (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  line_id BIGINT UNSIGNED NOT NULL,
  from_status VARCHAR(8) NOT NULL,
  to_status VARCHAR(8) NOT NULL,
  changed_at DATETIME(6) NOT NULL,
  actor_user_id BIGINT UNSIGNED NOT NULL,
  KEY ix_line_status_changes_line (line_id, changed_at, id),
  CONSTRAINT fk_line_status_changes_line FOREIGN KEY (line_id) REFERENCES app_lines (id) ON DELETE CASCADE,
  CONSTRAINT fk_line_status_changes_actor FOREIGN KEY (actor_user_id) REFERENCES users (id)
) ENGINE=InnoDB;
