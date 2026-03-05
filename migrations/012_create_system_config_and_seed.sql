-- =============================================================
-- Migration 012: System Config & Seed Data
-- BPR Perdana E-Form System
-- =============================================================

CREATE TABLE system_config (
    config_key      VARCHAR(100)    NOT NULL,
    config_value    TEXT            NOT NULL,
    description     VARCHAR(255)    NULL,
    is_public       TINYINT(1)      NOT NULL DEFAULT 0
                        COMMENT '1 = safe to expose to frontend (e.g. product names), 0 = internal only',
    updated_by      CHAR(36)        NULL COMMENT 'internal_users.id of last editor',
    updated_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at      DATETIME        NULL,

    PRIMARY KEY (config_key),
    CONSTRAINT fk_system_config_updated_by
        FOREIGN KEY (updated_by) REFERENCES internal_users(id)
        ON UPDATE CASCADE ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- -------------------------------------------------------------
-- Seed: Default system configuration values
-- These can be updated by Admin via the dashboard.
-- -------------------------------------------------------------
INSERT INTO system_config (config_key, config_value, description, is_public) VALUES

-- Deposit interest rates (%) per tenor
('deposit.rate.1_month',    '4.00',  'Deposito interest rate for 1-month tenor (%)',  1),
('deposit.rate.3_month',    '4.25',  'Deposito interest rate for 3-month tenor (%)',  1),
('deposit.rate.6_month',    '4.50',  'Deposito interest rate for 6-month tenor (%)',  1),
('deposit.rate.12_month',   '5.00',  'Deposito interest rate for 12-month tenor (%)', 1),

-- Loan interest rates (%) per product
('loan.rate.kmk',           '12.00', 'Kredit Modal Kerja annual interest rate (%)',    1),
('loan.rate.kag',           '14.00', 'Kredit Aneka Guna annual interest rate (%)',     1),

-- Savings minimum initial deposit
('saving.min_deposit.perdana',      '100000',  'Minimum initial deposit for Tabungan Perdana (IDR)',      1),
('saving.min_deposit.perdana_plus', '500000',  'Minimum initial deposit for Tabungan Perdana Plus (IDR)', 1),
('saving.min_deposit.tabunganku',   '20000',   'Minimum initial deposit for TabunganKu (IDR)',            1),

-- Application behavior
('app.sign_deadline_days',  '7',    'Days customer has to sign after approval before application expires', 0),
('app.session_ttl_minutes', '120',  'Customer session token TTL in minutes',           0),

-- Notification settings
('notif.esign_reminder_days', '3',  'Send reminder WhatsApp N days before sign deadline', 0),

-- VIDA integration toggles (useful for maintenance windows)
('vida.ocr.enabled',        '1',    '1 = enabled, 0 = temporarily disabled',           0),
('vida.liveness.enabled',   '1',    '1 = enabled, 0 = temporarily disabled',           0),
('vida.esign.enabled',      '1',    '1 = enabled, 0 = temporarily disabled',           0),
('vida.emeterai.enabled',   '1',    '1 = enabled, 0 = temporarily disabled',           0),

-- eKYC thresholds for admin review flagging
('kyc.ocr.min_confidence',         '0.70', 'OCR confidence below this triggers a flag in admin UI',         0),
('kyc.liveness.min_score',         '0.80', 'Liveness score below this triggers a flag in admin UI',         0),
('kyc.face_match.min_score',       '0.75', 'Face match score below this triggers a flag in admin UI',       0);
