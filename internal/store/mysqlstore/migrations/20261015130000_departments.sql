-- Organization (#25): the department tree, kept in this tool. It only displays and finds users;
-- it takes no part in authorization. parent_id is NULL for a top-level department.
-- source / external_id are reserved so a later release can sync departments from LDAP:
-- source is 'local' for everything today, external_id is unused.
-- RESTRICT (the default) on the parent: a department with children cannot be deleted.
CREATE TABLE departments (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  parent_id BIGINT UNSIGNED NULL,
  name VARCHAR(200) NOT NULL,
  source VARCHAR(16) NOT NULL DEFAULT 'local',
  external_id VARCHAR(255) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY ix_departments_parent (parent_id),
  UNIQUE KEY uq_departments_external (source, external_id),
  CONSTRAINT fk_departments_parent FOREIGN KEY (parent_id) REFERENCES departments (id)
) ENGINE=InnoDB;

-- One row to lock while the tree is being restructured (a move needs a consistent tree to
-- keep it free of cycles).
CREATE TABLE department_tree_lock (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY
) ENGINE=InnoDB;

INSERT INTO department_tree_lock (id) VALUES (1);
