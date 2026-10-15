-- Organization (#25): the department a user belongs to (at most one; NULL = none). Not the
-- same thing as users.department, which is the read-only text synced from LDAP.
-- RESTRICT: a department with members cannot be deleted.
ALTER TABLE users
  ADD COLUMN department_id BIGINT UNSIGNED NULL,
  ADD KEY ix_users_department (department_id),
  ADD CONSTRAINT fk_users_department FOREIGN KEY (department_id) REFERENCES departments (id);
