-- Audit log (#27): identity- and security-related operations. Append-only: the repository
-- port only appends and lists. An entry is an action TYPE plus structured PARAMS; the
-- frontend renders the sentence in the current UI language.
--
-- actor_kind is 'user' (actor_user_id set) or 'cli' (the command line in the container;
-- actor_user_id is NULL). target_type / target_id name the object: 'user' with the user ID,
-- or 'application' with the application key. There is deliberately no foreign key to
-- applications: deleting an application must not delete its audit entries.
CREATE TABLE audit_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  occurred_at DATETIME(6) NOT NULL,
  actor_kind VARCHAR(8) NOT NULL,
  actor_user_id BIGINT UNSIGNED NULL,
  action VARCHAR(64) NOT NULL,
  target_type VARCHAR(32) NOT NULL,
  target_id VARCHAR(128) NOT NULL,
  params JSON NOT NULL,
  KEY ix_audit_log_time (occurred_at, id),
  KEY ix_audit_log_actor (actor_user_id, occurred_at),
  KEY ix_audit_log_target (target_type, target_id, occurred_at),
  KEY ix_audit_log_action (action, occurred_at),
  CONSTRAINT fk_audit_log_actor FOREIGN KEY (actor_user_id) REFERENCES users (id)
) ENGINE=InnoDB;
