-- =============================================================
-- Migration 013: Additional Composite Indexes for Query Performance
-- BPR Perdana E-Form System
-- =============================================================

-- Admin dashboard: list applications filtered by status + product, sorted by date
-- Query: WHERE status = ? AND product_type = ? ORDER BY created_at DESC
ALTER TABLE applications
    ADD INDEX idx_apps_status_product_created (status, product_type, created_at);

-- Admin dashboard: applications pending review (most common filter)
-- Query: WHERE status IN ('PENDING_REVIEW','IN_REVIEW','RECOMMENDED') AND deleted_at IS NULL
ALTER TABLE applications
    ADD INDEX idx_apps_active_status (status, deleted_at);

-- Operator queue: applications assigned/opened by a specific operator
-- Query against review_actions: WHERE actor_id = ? AND action = 'OPENED'
ALTER TABLE review_actions
    ADD INDEX idx_review_actor_action (actor_id, action, created_at);

-- Audit log queries by compliance team: all events for a specific application
-- Query: WHERE entity_type = 'application' AND entity_id = ?
ALTER TABLE audit_logs
    ADD INDEX idx_audit_entity_created (entity_type, entity_id, created_at);

-- Audit log queries: all actions by a specific internal user
-- Query: WHERE actor_id = ? ORDER BY created_at DESC
ALTER TABLE audit_logs
    ADD INDEX idx_audit_actor_created (actor_id, created_at);

-- Sign deadline monitoring (background job to expire stale SIGNING applications)
-- Query: WHERE status = 'SIGNING' AND sign_deadline < NOW()
ALTER TABLE applications
    ADD INDEX idx_apps_signing_deadline (status, sign_deadline);

-- Notification retry job: find failed/pending notifications
-- Query: WHERE status IN ('PENDING','FAILED') AND retry_count < 3
ALTER TABLE notification_logs
    ADD INDEX idx_notif_status_retry (status, retry_count, created_at);

-- Webhook reprocessing: find unprocessed events
-- Query: WHERE processed = 0 AND process_attempts < 3 ORDER BY received_at ASC
ALTER TABLE webhook_events
    ADD INDEX idx_webhook_unprocessed (processed, process_attempts, received_at);

-- Customer lookup by NIK (identity deduplication check before allowing new application)
-- Query: WHERE nik = ? AND deleted_at IS NULL
ALTER TABLE customers
    ADD INDEX idx_customers_nik_active (nik, deleted_at);

-- Session lookup by token hash (every customer API request)
ALTER TABLE customer_sessions
    ADD INDEX idx_sessions_token_expires (token_hash, expires_at);
