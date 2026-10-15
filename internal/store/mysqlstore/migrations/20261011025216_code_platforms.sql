-- Code platforms (GitLab / GitHub instances) registered by administrators (ADR-0001).
-- token_enc holds the read-only service account token encrypted with AES-256-GCM
-- (nonce || ciphertext); the plaintext is never stored. token_tail is the display tail.

CREATE TABLE code_platforms (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  kind VARCHAR(16) NOT NULL,
  base_url VARCHAR(512) NOT NULL,
  token_enc VARBINARY(4096) NOT NULL,
  token_tail VARCHAR(8) NOT NULL,
  created_at DATETIME(6) NOT NULL
) ENGINE=InnoDB;
