-- The outgoing mail queue (#20). Mail is written here in the same transaction as the change
-- that causes it, and a background task delivers it and retries failures with a backoff.
--
-- status: pending (waiting for its next attempt), sent, failed (gave up after the last
-- attempt). The rendered subject and bodies are kept only while the mail is pending: they may
-- carry one-time links and codes, so delivery (or giving up) clears them.
-- locked_until is the lease of the worker that claimed the row, so several instances never
-- deliver the same mail twice at the same time; a crashed worker's lease just runs out.
CREATE TABLE mail_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  to_address VARCHAR(320) NOT NULL,
  recipient_user_id BIGINT NULL,
  template VARCHAR(64) NOT NULL,
  lang VARCHAR(8) NOT NULL,
  subject VARCHAR(998) NOT NULL,
  body_text MEDIUMTEXT NOT NULL,
  body_html MEDIUMTEXT NOT NULL,
  status VARCHAR(16) NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(6) NOT NULL,
  locked_until DATETIME(6) NULL,
  last_error_code VARCHAR(64) NOT NULL DEFAULT '',
  last_error VARCHAR(1000) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  sent_at DATETIME(6) NULL,
  failed_at DATETIME(6) NULL,
  KEY idx_mail_outbox_due (status, next_attempt_at)
) ENGINE=InnoDB;
