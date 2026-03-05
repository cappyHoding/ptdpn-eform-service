-- =============================================================
-- Migration 011: Audit Logs
-- BPR Perdana E-Form System
-- =============================================================

-- IMMUTABLE append-only table. NO soft deletes. NO updates.
-- Every row represents a single factual event that occurred.
-- The application layer must NEVER issue UPDATE or DELETE on this table.
--
-- actor_username and actor_role are snapshots (denormalized) so that
-- the audit record remains accurate even if the user is later deleted
-- or their role is changed — this is a hard OJK compliance requirement.
--
-- Predefined action values (extend as needed):
--
-- AUTH events:
--   LOGIN_SUCCESS         Internal user logged in successfully
--   LOGIN_FAILED          Failed login attempt (wrong password, inactive account)
--   LOGOUT                Internal user logged out
--   TOKEN_REFRESHED       JWT access token refreshed
--
-- APPLICATION events:
--   APP_CREATED           New application record created
--   APP_STEP_SAVED        Customer saved a form step
--   APP_SUBMITTED         Customer submitted (PENDING_REVIEW)
--   APP_STATUS_CHANGED    Any status transition (use old_value/new_value)
--   APP_EXPIRED           Application marked EXPIRED by system job
--
-- REVIEW events:
--   REVIEW_OPENED         Operator opened an application
--   REVIEW_RECOMMENDED    Operator recommended application to supervisor
--   REVIEW_APPROVED       Supervisor gave final approval
--   REVIEW_REJECTED       Operator or supervisor rejected
--
-- SIGNING events:
--   CONTRACT_GENERATED    PDF contract created by worker
--   EMETERAI_APPLIED      eMeterai stamp applied by VIDA
--   ESIGN_SENT            eSign link dispatched to customer
--   CONTRACT_SIGNED       VIDA webhook confirmed signature
--
-- USER MANAGEMENT events:
--   USER_CREATED          Admin created internal user
--   USER_DEACTIVATED      Admin deactivated internal user
--   USER_REACTIVATED      Admin reactivated internal user
--   USER_ROLE_CHANGED     Admin changed a user's role
--   CONFIG_UPDATED        Admin changed a system_config value

CREATE TABLE audit_logs (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    -- Who performed the action
    actor_type      ENUM('customer','internal_user','system') NOT NULL,
    actor_id        VARCHAR(36)     NULL  COMMENT 'UUID of customer or internal_user, null for system',
    actor_username  VARCHAR(100)    NULL  COMMENT 'Snapshot — not a FK',
    actor_role      VARCHAR(20)     NULL  COMMENT 'Snapshot of role at time of action',

    -- What happened
    action          VARCHAR(100)    NOT NULL,
    description     TEXT            NULL  COMMENT 'Human-readable summary of the event',

    -- What was affected
    entity_type     VARCHAR(50)     NULL  COMMENT 'e.g. application, internal_user, system_config',
    entity_id       VARCHAR(36)     NULL  COMMENT 'UUID or key of the affected record',

    -- State change evidence (for OJK compliance)
    old_value       JSON            NULL  COMMENT 'State before the change',
    new_value       JSON            NULL  COMMENT 'State after the change',

    -- Request context
    ip_address      VARCHAR(45)     NULL,
    user_agent      TEXT            NULL,
    request_id      VARCHAR(100)    NULL  COMMENT 'Correlation ID from HTTP header X-Request-ID',

    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- NO updated_at. NO deleted_at. This table is immutable.

    PRIMARY KEY (id),
    KEY idx_audit_logs_actor_id     (actor_id),
    KEY idx_audit_logs_action       (action),
    KEY idx_audit_logs_entity       (entity_type, entity_id),
    KEY idx_audit_logs_created      (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
