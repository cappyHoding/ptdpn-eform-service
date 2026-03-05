-- =============================================================
-- Migration 008: Contract Documents
-- BPR Perdana E-Form System
-- =============================================================

-- Tracks the generated PDF contract and its signing lifecycle.
-- Created by the background worker after Supervisor approval.
--
-- PDF stored at: /var/app/storage/contracts/{year}/{month}/{application_id}.pdf
--
-- Lifecycle of sign_status:
--   PENDING  → PDF generated, eMeterai applied, not yet sent
--   SENT     → eSign link dispatched to customer via Email + WhatsApp
--   SIGNED   → VIDA webhook confirmed customer has signed
--   EXPIRED  → Customer did not sign before sign_deadline

CREATE TABLE contract_documents (
    id                  CHAR(36)        NOT NULL,
    application_id      CHAR(36)        NOT NULL,
    document_type       ENUM(
                            'SAVINGS_AGREEMENT',
                            'DEPOSIT_AGREEMENT',
                            'LOAN_AGREEMENT'
                        )               NOT NULL,

    -- Local file storage
    file_path           VARCHAR(500)    NOT NULL
                            COMMENT '/var/app/storage/contracts/{year}/{month}/{application_id}.pdf',
    file_size_bytes     INT UNSIGNED    NULL,
    file_hash_sha256    CHAR(64)        NULL COMMENT 'Integrity check for the generated PDF',

    -- VIDA eSign (POA) references
    vida_document_id    VARCHAR(100)    NULL COMMENT 'Document ID from VIDA after registration',
    vida_sign_request_id VARCHAR(100)  NULL COMMENT 'Signing request ID from VIDA POA API',

    -- VIDA eMeterai reference
    emeterai_id         VARCHAR(100)    NULL COMMENT 'eMeterai stamp ID from VIDA',
    emeterai_applied_at DATETIME        NULL,

    -- Signing lifecycle
    sign_status         ENUM('PENDING','SENT','SIGNED','EXPIRED') NOT NULL DEFAULT 'PENDING',
    sign_link           VARCHAR(500)    NULL COMMENT 'URL sent to customer to complete signing',
    sign_link_sent_at   DATETIME        NULL,
    sign_deadline       DATETIME        NULL COMMENT 'Auto-set to 7 days after sign_link_sent_at',
    signed_at           DATETIME        NULL COMMENT 'Timestamp from VIDA webhook confirmation',

    -- Signed document (VIDA returns a signed PDF URL or we download and store locally)
    signed_file_path    VARCHAR(500)    NULL
                            COMMENT '/var/app/storage/contracts/{year}/{month}/{application_id}_signed.pdf',

    generated_at        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at          DATETIME        NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uq_contract_documents_application (application_id),
    KEY        idx_contract_sign_status          (sign_status),

    CONSTRAINT fk_contract_documents_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
