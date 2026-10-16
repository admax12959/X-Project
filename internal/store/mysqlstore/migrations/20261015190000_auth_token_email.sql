-- A password-setting token (invitation, recovery) is for the login email it was mailed to: the
-- column holds that address, and the token stops working once the user's email identity has
-- another login name or is gone. NULL for tokens that are not tied to an address.
ALTER TABLE auth_tokens ADD COLUMN login_email VARCHAR(255) NULL;
