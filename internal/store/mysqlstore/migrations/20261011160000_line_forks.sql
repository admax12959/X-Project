-- Maintenance lines (#9): the fork facts and the lifecycle state of a line.
-- The mainline row keeps source_version_id, reason, status, planned_end and forked_at NULL.
-- status is 'active', 'frozen' or 'retired' (lowercase, like the other enums).
-- source_version_id has an index but deliberately no foreign key: app_versions already
-- references app_lines, and a key back (RESTRICT or CASCADE) would make deleting an
-- application depend on the order of two cascades. The domain checks that the source
-- version belongs to the same application before inserting; it is never changed afterwards.

ALTER TABLE app_lines
  ADD COLUMN source_version_id BIGINT UNSIGNED NULL,
  ADD COLUMN reason TEXT NULL,
  ADD COLUMN status VARCHAR(8) NULL,
  ADD COLUMN planned_end DATE NULL,
  ADD COLUMN note TEXT NULL,
  ADD COLUMN forked_at DATETIME(6) NULL,
  ADD KEY ix_app_lines_source (source_version_id);
