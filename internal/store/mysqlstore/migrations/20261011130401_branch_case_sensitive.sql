-- Branch names are case-sensitive: Cust/X and cust/x are two different branches, exactly
-- like version numbers (20261011140000). The unique key (application_id, branch) is
-- rebuilt with the binary collation.
ALTER TABLE app_lines MODIFY branch VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;
