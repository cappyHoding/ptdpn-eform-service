-- =============================================================
-- Migration 009: Webhook Events
-- BPR Perdana E-Form System
-- =============================================================

-- Persists every incoming webhook payload from VIDA.
-- Designed for idempotency: before processing, check if
-- vida_event_id already exists. If yes, skip processing.
--
-- Two main webhook events expected from VIDA:
--   document.signed      → Customer completed e-signature
--   document.expired     → Signing session expired
--
-- Raw payload is always stored regardless of processing outcome.
-- This allows manual replay if a processing error occurs.

CREATE TABLE webhook_events (
    id              CHAR(36)        NOT NULL,
    source          VARCHAR(50)     NOT NULL DEFAULT 'vida'
                        COMMENT 'Origin system sending the webhook',
    event_type      VARCHAR(100)    NOT NULL
                        COMMENT 'e.g. document.signed, document.expired',

    -- Link to our application (parsed from payload, may be null if parsing fails)
    application_id  CHAR(36)        NULL,

    -- Idempotency key: VIDA sends a unique event ID per webhook call
    vida_event_id   VARCHAR(100)    NULL
                        COMMENT 'VIDA unique event ID for idempotency — check before processing',

    -- Full raw payload — never modify this after insert
    payload         JSON            NOT NULL,

    -- Processing state
    processed       TINYINT(1)      NOT NULL DEFAULT 0,
    processed_at    DATETIME        NULL,
    process_attempts TINYINT UNSIGNED NOT NULL DEFAULT 0
                        COMMENT 'Incremented on each processing attempt',
    error_message   TEXT            NULL COMMENT 'Last processing error if any',

    received_at     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      DATETIME        NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uq_webhook_events_vida_event_id  (vida_event_id),
    KEY        idx_webhook_events_application   (application_id),
    KEY        idx_webhook_events_processed     (processed),
    KEY        idx_webhook_events_event_type    (event_type),
    KEY        idx_webhook_events_received      (received_at)

    -- Intentionally no FK on application_id:
    -- We must persist the payload even if application lookup fails.
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
