-- Versions (#8): manually registered releases. Each belongs to exactly one line.
-- The number is unique per application and compared exactly (utf8mb4_bin): Git tags
-- are case-sensitive and the format is not restricted, so V1.0 and v1.0 are different.
-- ref_kind is 'tag' or 'commit'; ref_value is stored as typed and never verified (ADR-0001).
-- released_at is a calendar date (no time of day).

CREATE TABLE app_versions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  application_id BIGINT UNSIGNED NOT NULL,
  line_id BIGINT UNSIGNED NOT NULL,
  number VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  ref_kind VARCHAR(6) NOT NULL,
  ref_value VARCHAR(255) NOT NULL,
  released_at DATE NOT NULL,
  note TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  created_by BIGINT UNSIGNED NOT NULL,
  UNIQUE KEY uq_app_versions_number (application_id, number),
  KEY ix_app_versions_line (line_id, released_at),
  CONSTRAINT fk_app_versions_app FOREIGN KEY (application_id) REFERENCES applications (id) ON DELETE CASCADE,
  CONSTRAINT fk_app_versions_line FOREIGN KEY (line_id) REFERENCES app_lines (id) ON DELETE CASCADE,
  CONSTRAINT fk_app_versions_creator FOREIGN KEY (created_by) REFERENCES users (id)
) ENGINE=InnoDB;
