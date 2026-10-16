-- Avatars (#32, ADR-0007): the one current avatar object of a user. avatar_key is the object
-- key in the object store, avatar_hash (first 16 hex digits of the SHA-256 of the stored image)
-- is what the URL carries to bust the browser cache, avatar_source is who put it there:
-- 'upload' (the user) or 'ldap' (the directory photo). All NULL = no avatar.
ALTER TABLE users
  ADD COLUMN avatar_key VARCHAR(255) NULL,
  ADD COLUMN avatar_hash CHAR(16) NULL,
  ADD COLUMN avatar_source VARCHAR(8) NULL;
