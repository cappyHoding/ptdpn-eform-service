-- =============================================================
-- Migration 002: Customers & Customer Sessions
-- BPR Perdana E-Form System
-- =============================================================

CREATE TABLE customers (
    id                  CHAR(36)        NOT NULL,
    nik                 VARCHAR(16)     NULL    COMMENT 'Populated after OCR Step 3',
    full_name           VARCHAR(100)    NULL    COMMENT 'Populated after OCR Step 3',
    mothers_maiden_name VARCHAR(100)    NULL    COMMENT 'Filled in Step 4 - Additional Data',
    current_address     TEXT            NULL    COMMENT 'Populated after OCR Step 3',
    occupation          VARCHAR(100)    NULL    COMMENT 'Filled in Step 4 - Additional Data',
    work_duration       VARCHAR(50)     NULL    COMMENT 'e.g. "2 tahun 3 bulan"',
    monthly_income      BIGINT UNSIGNED NULL    COMMENT 'Stored in IDR (Rupiah), no decimals',
    education           ENUM(
                            'SD','SMP','SMA/SMK',
                            'D1','D2','D3','D4',
                            'S1','S2','S3','Lainnya'
                        )               NULL,
    email               VARCHAR(100)    NULL,
    phone_number        VARCHAR(20)     NULL,
    phone_number_wa     VARCHAR(20)     NULL    COMMENT 'WhatsApp number, may differ from phone_number',
    work_address        TEXT            NULL,
    created_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at          DATETIME        NULL,
    PRIMARY KEY (id),
    KEY idx_customers_nik        (nik),
    KEY idx_customers_email      (email),
    KEY idx_customers_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- Tracks anonymous customer sessions between form steps.
-- A new token is issued when a customer resumes an in-progress application.
-- Customers authenticate each step request using this token (not a password).
CREATE TABLE customer_sessions (
    id              CHAR(36)        NOT NULL,
    application_id  CHAR(36)        NOT NULL,
    token           VARCHAR(512)    NOT NULL COMMENT 'Signed JWT or opaque token, hashed before storage',
    token_hash      CHAR(64)        NOT NULL COMMENT 'SHA-256 of raw token for lookup',
    ip_address      VARCHAR(45)     NULL     COMMENT 'Supports both IPv4 and IPv6',
    user_agent      TEXT            NULL,
    expires_at      DATETIME        NOT NULL,
    revoked_at      DATETIME        NULL     COMMENT 'Set when customer completes step or session is invalidated',
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      DATETIME        NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_customer_sessions_token_hash (token_hash),
    KEY        idx_customer_sessions_app_id    (application_id),
    KEY        idx_customer_sessions_expires   (expires_at)
    -- FK to applications added in migration 003 to avoid circular dependency
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
