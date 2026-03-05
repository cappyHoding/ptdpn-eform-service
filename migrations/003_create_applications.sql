-- =============================================================
-- Migration 003: Applications (Core State Machine Table)
-- BPR Perdana E-Form System
-- =============================================================

CREATE TABLE applications (
    id                      CHAR(36)    NOT NULL,
    customer_id             CHAR(36)    NOT NULL,
    product_type            ENUM('SAVING','DEPOSIT','LOAN') NOT NULL,

    -- State machine: tracks lifecycle of the onboarding application
    status                  ENUM(
                                'DRAFT',            -- Customer started but not submitted
                                'PENDING_REVIEW',   -- Customer submitted, awaiting operator
                                'IN_REVIEW',        -- Operator has opened the application
                                'RECOMMENDED',      -- Operator approved, awaiting supervisor
                                'APPROVED',         -- Supervisor gave final approval
                                'REJECTED',         -- Rejected by operator or supervisor
                                'SIGNING',          -- PDF generated, eSign link sent to customer
                                'COMPLETED',        -- Customer signed, process done
                                'EXPIRED'           -- Application timed out (e.g. no sign within 7 days)
                            ) NOT NULL DEFAULT 'DRAFT',

    -- Step tracking: allows customers to resume their form
    current_step            TINYINT UNSIGNED NOT NULL DEFAULT 1
                                COMMENT '1=Agreement, 2=Product, 3=OCR, 4=AddData, 5=Liveness, 6=Disbursement, 7=Summary, 8=Sign',
    last_step_completed     TINYINT UNSIGNED NOT NULL DEFAULT 0
                                COMMENT '0 means no step completed yet',

    -- Step 1: Agreement — stored directly on application for legal evidence
    agreement_accepted      TINYINT(1)  NOT NULL DEFAULT 0,
    agreement_accepted_at   DATETIME    NULL,
    agreement_ip            VARCHAR(45) NULL,
    agreement_user_agent    TEXT        NULL COMMENT 'Browser fingerprint for legal record',

    -- Step 7: Submission
    submitted_at            DATETIME    NULL,

    -- Internal review outcome
    rejection_reason        TEXT        NULL COMMENT 'Set when status = REJECTED',
    completed_at            DATETIME    NULL COMMENT 'Set when status = COMPLETED',

    -- Application expiry (for SIGNING state timeout)
    sign_deadline           DATETIME    NULL COMMENT 'Deadline for customer to sign after approval',

    created_at              DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at              DATETIME    NULL,

    PRIMARY KEY (id),
    KEY idx_applications_customer   (customer_id),
    KEY idx_applications_status     (status),
    KEY idx_applications_product    (product_type),
    KEY idx_applications_created    (created_at),
    KEY idx_applications_deleted    (deleted_at),

    CONSTRAINT fk_applications_customer
        FOREIGN KEY (customer_id) REFERENCES customers(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- Now that applications exists, add the FK on customer_sessions
ALTER TABLE customer_sessions
    ADD CONSTRAINT fk_customer_sessions_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE;
