-- Interface language preference, saved on the user so it follows them to other devices.
-- Empty = never chosen.
ALTER TABLE users ADD COLUMN language VARCHAR(8) NOT NULL DEFAULT '';
