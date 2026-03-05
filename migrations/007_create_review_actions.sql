-- =============================================================
-- Migration 007: Review Actions (Maker-Checker Audit Trail)
-- BPR Perdana E-Form System
-- =============================================================

-- Every action an internal user takes on an application is recorded here.
-- This table provides the full four-eyes principle audit trail:
--   - Who opened it (operator)
--   - Who recommended it (operator/maker)
--   - Who approved or rejected it (supervisor/checker)
--   - Any notes added at each stage
--
-- IMPORTANT: Records in this table should never be deleted.
-- deleted_at is present only for schema consistency but should
-- not be used in business logic. Consider enforcing this via
-- application-level policy.

CREATE TABLE review_actions (
    id              CHAR(36)    NOT NULL,
    application_id  CHAR(36)    NOT NULL,
    actor_id        CHAR(36)    NOT NULL COMMENT 'internal_users.id',
    actor_username  VARCHAR(50) NOT NULL COMMENT 'Snapshot at time of action — survives user deletion',
    actor_role      ENUM('admin','supervisor','operator') NOT NULL
                        COMMENT 'Snapshot of role at time of action',
    action          ENUM(
                        'OPENED',       -- Operator opened and started reviewing
                        'RECOMMENDED',  -- Operator (Maker) passed to Supervisor
                        'APPROVED',     -- Supervisor (Checker) gave final approval
                        'REJECTED',     -- Operator or Supervisor rejected
                        'NOTE_ADDED',   -- Internal note added without status change
                        'REOPENED'      -- Admin reopened a rejected application
                    ) NOT NULL,
    notes           TEXT        NULL COMMENT 'Mandatory for REJECTED and NOTE_ADDED actions',
    created_at      DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      DATETIME    NULL,

    PRIMARY KEY (id),
    KEY idx_review_actions_application  (application_id),
    KEY idx_review_actions_actor        (actor_id),
    KEY idx_review_actions_action       (action),
    KEY idx_review_actions_created      (created_at),

    CONSTRAINT fk_review_actions_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE,
    CONSTRAINT fk_review_actions_actor
        FOREIGN KEY (actor_id) REFERENCES internal_users(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
