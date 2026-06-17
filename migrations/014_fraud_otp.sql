-- Tambah fraud verification tracking di liveness_results
ALTER TABLE liveness_results
    ADD COLUMN fraud_status VARCHAR(10) NOT NULL DEFAULT '001'
        COMMENT '001=in_progress, 002=submitted_manual, 003=accepted, 004=rejected, 006=cert_not_issued, 007=cert_issued'
        AFTER face_match_score,
    ADD COLUMN kyc_event_id VARCHAR(100) NULL
        COMMENT 'VIDA event ID setelah fraud status 003 — dipakai sebagai kyc_event_id di Direct Sign'
        AFTER fraud_status,
    ADD INDEX idx_liveness_fraud_status (fraud_status);

-- Tambah kolom OTP di customers
ALTER TABLE customers
    ADD COLUMN phone_verified    TINYINT(1) NOT NULL DEFAULT 0
        COMMENT '1 jika nomor HP sudah diverifikasi via OTP'
        AFTER phone_number_wa,
    ADD COLUMN phone_verified_at DATETIME NULL
        AFTER phone_verified;

-- Tambah status FRAUD_REJECTED di applications
ALTER TABLE applications
    MODIFY COLUMN status ENUM(
        'DRAFT',
        'PENDING_REVIEW',
        'IN_REVIEW',
        'RECOMMENDED',
        'APPROVED',
        'REJECTED',
        'FRAUD_REJECTED',
        'SIGNING',
        'COMPLETED',
        'EXPIRED'
    ) NOT NULL DEFAULT 'DRAFT';