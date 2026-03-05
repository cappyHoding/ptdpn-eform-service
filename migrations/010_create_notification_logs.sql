-- =============================================================
-- Migration 010: Notification Logs
-- BPR Perdana E-Form System
-- =============================================================

-- Tracks every outbound notification sent to customers.
-- Covers both Email and WhatsApp channels.
--
-- Common notification triggers:
--   esign_link_email    → eSign link sent after approval (Email)
--   esign_link_wa       → eSign link sent after approval (WhatsApp)
--   rejection_email     → Application rejected notification
--   rejection_wa        → Application rejected notification (WhatsApp)
--   completed_email     → Confirmation after signing complete
--   completed_wa        → Confirmation after signing complete (WhatsApp)
--   reminder_wa         → Reminder if customer hasn't signed yet

CREATE TABLE notification_logs (
    id              CHAR(36)        NOT NULL,
    application_id  CHAR(36)        NOT NULL,
    channel         ENUM('EMAIL','WHATSAPP') NOT NULL,
    recipient       VARCHAR(200)    NOT NULL
                        COMMENT 'Email address or WhatsApp phone number (with country code)',
    template        VARCHAR(100)    NOT NULL COMMENT 'Template identifier, e.g. esign_link_email',
    subject         VARCHAR(255)    NULL     COMMENT 'Email subject line (null for WhatsApp)',
    status          ENUM('PENDING','SENT','FAILED') NOT NULL DEFAULT 'PENDING',
    provider_message_id VARCHAR(200) NULL    COMMENT 'Message ID returned by Email/WA provider',
    sent_at         DATETIME        NULL,
    error_message   TEXT            NULL COMMENT 'Provider error response if status = FAILED',
    retry_count     TINYINT UNSIGNED NOT NULL DEFAULT 0,
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at      DATETIME        NULL,

    PRIMARY KEY (id),
    KEY idx_notification_logs_application   (application_id),
    KEY idx_notification_logs_channel       (channel),
    KEY idx_notification_logs_status        (status),

    CONSTRAINT fk_notification_logs_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
