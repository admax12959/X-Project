-- The local identity becomes the email identity (#29): it logs in with the login
-- email instead of the username.
--
-- Existing local identities are converted in place. The login email is the user's email
-- when that address is non-empty and held by exactly one converted user; every other
-- user gets the placeholder <username>@x-project.invalid, which an administrator then
-- replaces with the real address (the .invalid TLD can never receive mail).
--
-- A contact email in the placeholder domain is never taken as a login email: it could equal
-- the placeholder of another user and break uq_identity_login half way through. MySQL DDL
-- commits as it goes, so the two ALTERs check for their column first and the file can run
-- again after a failure.

-- A password an administrator chose must be changed: the flag sits on the identity until
-- it is, and on every session opened with it (the user can do nothing else in such a session).
SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE login_identities ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT FALSE', 'DO 0') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'login_identities' AND COLUMN_NAME = 'must_change_password');
PREPARE ddl_stmt FROM @ddl;
EXECUTE ddl_stmt;
DEALLOCATE PREPARE ddl_stmt;
SET @ddl = (SELECT IF(COUNT(*) = 0, 'ALTER TABLE sessions ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT FALSE', 'DO 0') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sessions' AND COLUMN_NAME = 'must_change_password');
PREPARE ddl_stmt FROM @ddl;
EXECUTE ddl_stmt;
DEALLOCATE PREPARE ddl_stmt;

UPDATE login_identities li
  JOIN users u ON u.id = li.user_id
   SET li.kind = 'email', li.login_name = CONCAT(u.username, '@x-project.invalid')
 WHERE li.kind = 'local';

-- The contact email is copied trimmed (an LDAP sync stores it as the directory had it, and the
-- login trims what it is given), and only when it fits the 255 characters of login_name and
-- looks like an address; every other user keeps the placeholder.
UPDATE login_identities li
  JOIN users u ON u.id = li.user_id
  JOIN (SELECT LOWER(TRIM(u2.email)) AS addr
          FROM login_identities l2 JOIN users u2 ON u2.id = l2.user_id
         WHERE l2.kind = 'email'
           AND TRIM(u2.email) <> ''
           AND CHAR_LENGTH(TRIM(u2.email)) <= 255
           AND LOWER(TRIM(u2.email)) NOT LIKE '%@x-project.invalid'
           AND TRIM(u2.email) REGEXP '^[^@[:space:],]+@[^@[:space:],]+[.][^@[:space:],]+$'
         GROUP BY LOWER(TRIM(u2.email))
        HAVING COUNT(*) = 1) uniq ON uniq.addr = LOWER(TRIM(u.email))
   SET li.login_name = uniq.addr
 WHERE li.kind = 'email';
