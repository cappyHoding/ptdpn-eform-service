-- =============================================================
-- Migration 005: Disbursement Data
-- BPR Perdana E-Form System
-- =============================================================

-- Stores the customer's external bank account for fund disbursement.
-- Required for LOAN and DEPOSIT product types (Step 6).
-- Optional for SAVING (customer may skip or provide for convenience).
--
-- bank_code follows Bank Indonesia standard codes used in
-- SKNBI (Sistem Kliring Nasional BI) and BI-FAST routing.
-- Reference: https://www.bi.go.id/id/sistem-pembayaran/ritel/sknbi

CREATE TABLE disbursement_data (
    id              CHAR(36)        NOT NULL,
    application_id  CHAR(36)        NOT NULL,
    bank_name       VARCHAR(100)    NOT NULL COMMENT 'Human-readable bank name, e.g. Bank Central Asia',
    bank_code       VARCHAR(10)     NOT NULL COMMENT 'BI standard routing code, e.g. 014 for BCA',
    account_number  VARCHAR(30)     NOT NULL,
    account_holder  VARCHAR(100)    NOT NULL COMMENT 'Name as printed on the bank account',
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at      DATETIME        NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_disbursement_application (application_id),
    CONSTRAINT fk_disbursement_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
