-- =============================================================
-- Migration 006: eKYC Results
-- (ocr_results and liveness_results as separate tables)
-- BPR Perdana E-Form System
-- =============================================================

-- -------------------------------------------------------------
-- OCR RESULTS (Step 3: KTP Upload)
-- Populated when backend calls VIDA KTP OCR API.
-- vida_request_id is the permanent audit trail to VIDA's records.
-- KTP image stored at: /var/app/storage/ktp/{year}/{month}/{application_id}.jpg
-- -------------------------------------------------------------
CREATE TABLE ocr_results (
    id                  CHAR(36)        NOT NULL,
    application_id      CHAR(36)        NOT NULL,

    -- VIDA transaction reference — critical for OJK audit
    vida_request_id     VARCHAR(100)    NOT NULL COMMENT 'VIDA transaction ID, permanent audit link',
    raw_response        JSON            NOT NULL COMMENT 'Full VIDA API response, never modified',

    -- Extracted KTP fields
    nik                 VARCHAR(16)     NULL,
    full_name           VARCHAR(100)    NULL,
    birth_place         VARCHAR(100)    NULL,
    birth_date          DATE            NULL,
    gender              ENUM('LAKI-LAKI','PEREMPUAN') NULL,
    address             TEXT            NULL,
    rt_rw               VARCHAR(10)     NULL COMMENT 'e.g. "003/007"',
    kelurahan           VARCHAR(100)    NULL,
    kecamatan           VARCHAR(100)    NULL,
    kabupaten_kota      VARCHAR(100)    NULL,
    provinsi            VARCHAR(100)    NULL,
    religion            ENUM(
                            'ISLAM','KRISTEN','KATOLIK',
                            'HINDU','BUDDHA','KONGHUCU','Lainnya'
                        )               NULL,
    marital_status      ENUM(
                            'BELUM KAWIN','KAWIN',
                            'CERAI HIDUP','CERAI MATI'
                        )               NULL,
    occupation          VARCHAR(100)    NULL,
    nationality         VARCHAR(50)     NULL DEFAULT 'WNI',
    expiry_date         DATE            NULL COMMENT 'NULL = SEUMUR HIDUP',

    -- Quality indicators surfaced in admin review UI
    confidence_score    DECIMAL(5,4)    NULL COMMENT 'Overall OCR confidence, e.g. 0.9823',
    nik_confidence      DECIMAL(5,4)    NULL,
    name_confidence     DECIMAL(5,4)    NULL,

    -- Local file path
    ktp_image_path      VARCHAR(500)    NOT NULL
                            COMMENT '/var/app/storage/ktp/{year}/{month}/{application_id}.jpg',

    created_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at          DATETIME        NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uq_ocr_results_application   (application_id),
    UNIQUE KEY uq_ocr_results_vida_req      (vida_request_id),
    KEY        idx_ocr_results_nik          (nik),

    CONSTRAINT fk_ocr_results_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- -------------------------------------------------------------
-- LIVENESS RESULTS (Step 5: Liveness Check)
-- Populated when VIDA Web SDK completes and frontend posts result
-- to our backend, which then validates with VIDA server-side.
-- Selfie stored at: /var/app/storage/selfie/{year}/{month}/{application_id}.jpg
-- -------------------------------------------------------------
CREATE TABLE liveness_results (
    id                  CHAR(36)        NOT NULL,
    application_id      CHAR(36)        NOT NULL,

    -- VIDA transaction references
    vida_request_id     VARCHAR(100)    NOT NULL COMMENT 'VIDA server-side verification ID',
    vida_session_id     VARCHAR(100)    NULL     COMMENT 'VIDA Web SDK session token',
    raw_response        JSON            NOT NULL COMMENT 'Full VIDA API response, never modified',

    -- Liveness verdict
    liveness_status     ENUM('PASSED','FAILED','REVIEW') NOT NULL,
    liveness_score      DECIMAL(5,4)    NULL COMMENT 'Anti-spoofing score, e.g. 0.9985',

    -- Face match verdict (compares live face to KTP photo from OCR)
    face_match_status   ENUM('MATCHED','NOT_MATCHED','REVIEW') NULL,
    face_match_score    DECIMAL(5,4)    NULL COMMENT 'Similarity score between selfie and KTP photo',

    -- Local file path
    selfie_image_path   VARCHAR(500)    NULL
                            COMMENT '/var/app/storage/selfie/{year}/{month}/{application_id}.jpg',

    created_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at          DATETIME        NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uq_liveness_results_application (application_id),
    UNIQUE KEY uq_liveness_results_vida_req    (vida_request_id),

    CONSTRAINT fk_liveness_results_application
        FOREIGN KEY (application_id) REFERENCES applications(id)
        ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
