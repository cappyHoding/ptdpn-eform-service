-- =============================================================
-- Migration 004: Product Detail Tables
-- (saving_details, deposit_details, loan_details, collateral_items)
-- BPR Perdana E-Form System
-- =============================================================

-- -------------------------------------------------------------
-- SAVING DETAILS
-- Only one row per application. Created in Step 2 when product_type = SAVING.
-- -------------------------------------------------------------
CREATE TABLE saving_details (
    application_id  CHAR(36)        NOT NULL,
    product_name    ENUM(
                        'Tabungan Perdana',
                        'Tabungan Perdana Plus',
                        'TabunganKu'
                    )               NOT NULL,
    initial_deposit BIGINT UNSIGNED NOT NULL COMMENT 'In IDR',
    source_of_funds VARCHAR(100)    NOT NULL,
    saving_purpose  VARCHAR(200)    NOT NULL COMMENT 'e.g. Tabungan Rutin, Dana Darurat',
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at      DATETIME        NULL,
    PRIMARY KEY (application_id),
    CONSTRAINT fk_saving_details_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- -------------------------------------------------------------
-- DEPOSIT DETAILS
-- Only one row per application. Created in Step 2 when product_type = DEPOSIT.
-- -------------------------------------------------------------
CREATE TABLE deposit_details (
    application_id      CHAR(36)        NOT NULL,
    product_name        VARCHAR(100)    NOT NULL DEFAULT 'Deposito Perdana',
    placement_amount    BIGINT UNSIGNED NOT NULL COMMENT 'In IDR',
    tenor_months        TINYINT UNSIGNED NOT NULL COMMENT 'Must be 1, 3, 6, or 12',
    rollover_type       ENUM('ARO','NON_ARO') NOT NULL
                            COMMENT 'ARO = Automatic Roll Over, NON_ARO = Tidak Diperpanjang',
    interest_rate       DECIMAL(5,2)    NULL
                            COMMENT 'Snapshot of rate at time of application from system_config',
    source_of_funds     VARCHAR(100)    NOT NULL,
    investment_purpose  VARCHAR(200)    NULL,
    created_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at          DATETIME        NULL,
    PRIMARY KEY (application_id),
    CONSTRAINT chk_deposit_tenor
        CHECK (tenor_months IN (1, 3, 6, 12)),
    CONSTRAINT fk_deposit_details_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- -------------------------------------------------------------
-- LOAN DETAILS
-- Only one row per application. Created in Step 2 when product_type = LOAN.
-- -------------------------------------------------------------
CREATE TABLE loan_details (
    application_id      CHAR(36)        NOT NULL,
    product_name        ENUM(
                            'Kredit Modal Kerja',
                            'Kredit Aneka Guna'
                        )               NOT NULL,
    requested_amount    BIGINT UNSIGNED NOT NULL COMMENT 'In IDR',
    tenor_months        TINYINT UNSIGNED NOT NULL COMMENT 'Loan repayment period in months',
    interest_rate       DECIMAL(5,2)    NULL
                            COMMENT 'Snapshot of rate from system_config at application time',
    loan_purpose        TEXT            NOT NULL COMMENT 'Applicant description of how funds will be used',
    payment_source      VARCHAR(200)    NOT NULL COMMENT 'How the applicant plans to repay, e.g. salary',
    source_of_funds     VARCHAR(100)    NOT NULL,
    created_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at          DATETIME        NULL,
    PRIMARY KEY (application_id),
    CONSTRAINT fk_loan_details_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- -------------------------------------------------------------
-- COLLATERAL ITEMS
-- One-to-many: a loan can have multiple collateral assets pledged.
-- Only relevant for product_type = LOAN.
-- File stored at: /var/app/storage/collateral/{year}/{month}/{filename}
-- -------------------------------------------------------------
CREATE TABLE collateral_items (
    id                  CHAR(36)        NOT NULL,
    application_id      CHAR(36)        NOT NULL,
    collateral_type     ENUM(
                            'SHM',          -- Sertifikat Hak Milik (land certificate)
                            'SHGB',         -- Sertifikat Hak Guna Bangunan
                            'BPKB',         -- Vehicle ownership document
                            'Deposito',     -- Deposito as collateral
                            'Lainnya'
                        )               NOT NULL,
    description         TEXT            NULL  COMMENT 'e.g. "Toyota Avanza 2019, Plat B 1234 XY"',
    estimated_value     BIGINT UNSIGNED NULL  COMMENT 'In IDR, assessed by applicant',
    attachment_path     VARCHAR(500)    NULL
                            COMMENT 'Local path: /var/app/storage/collateral/{year}/{month}/{filename}',
    sort_order          TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT 'Display order in admin UI',
    created_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at          DATETIME        NULL,
    PRIMARY KEY (id),
    KEY idx_collateral_application (application_id),
    CONSTRAINT fk_collateral_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
