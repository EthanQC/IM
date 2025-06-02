-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied.

CREATE DATABASE IF NOT EXISTS auth_service
  DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE auth_service;

-- 访问令牌表  auth_tokens
CREATE TABLE IF NOT EXISTS auth_tokens (
  jti            CHAR(36) PRIMARY KEY,           -- Access-Token 的 UUID
  user_id        BIGINT UNSIGNED NOT NULL,       -- 来自 User-Service
  refresh_jti    CHAR(36) NOT NULL,              -- Refresh-Token 的 UUID
  expires_at     TIMESTAMP NOT NULL,
  refresh_exp_at TIMESTAMP NOT NULL,
  is_revoked     BOOLEAN DEFAULT FALSE,
  roles          JSON NOT NULL,                  -- 按 Domain 的 []Role 序列化
  created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_tokens_user (user_id, is_revoked)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 封禁 / 黑名单表  user_status
CREATE TABLE IF NOT EXISTS user_status (
  user_id       BIGINT UNSIGNED PRIMARY KEY,
  is_blocked    BOOLEAN DEFAULT FALSE,
  block_reason  VARCHAR(100),
  blocked_at    TIMESTAMP NULL,
  block_exp_at  TIMESTAMP NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 路由白名单 / 规则表  access_rules
CREATE TABLE IF NOT EXISTS access_rules (
  id        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  path      VARCHAR(200) NOT NULL,
  pattern   VARCHAR(200),
  methods   SET('GET','POST','PUT','DELETE','PATCH','OPTIONS') NOT NULL,
  is_public BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_access_path (path)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back.
