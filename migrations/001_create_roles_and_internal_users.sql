-- =============================================================
-- Migration 001: Roles & Internal Users
-- BPR Perdana E-Form System
-- =============================================================

CREATE TABLE roles (
    id          TINYINT UNSIGNED    NOT NULL AUTO_INCREMENT,
    name        ENUM('admin','supervisor','operator') NOT NULL,
    description VARCHAR(100)        NULL,
    created_at  DATETIME            NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME            NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at  DATETIME            NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_roles_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Seed default roles immediately
INSERT INTO roles (name, description) VALUES
    ('admin',      'System administrator: manages users, config, and audit logs'),
    ('supervisor', 'Checker: performs final approval of recommended applications'),
    ('operator',   'Maker: performs initial review and recommends applications');


CREATE TABLE internal_users (
    id          CHAR(36)        NOT NULL,
    username    VARCHAR(50)     NOT NULL,
    full_name   VARCHAR(100)    NOT NULL,
    email       VARCHAR(100)    NOT NULL,
    password    VARCHAR(255)    NOT NULL COMMENT 'bcrypt hashed',
    role_id     TINYINT UNSIGNED NOT NULL,
    is_active   TINYINT(1)      NOT NULL DEFAULT 1,
    created_by  CHAR(36)        NULL COMMENT 'internal_users.id of admin who created this account',
    created_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at  DATETIME        NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_internal_users_username (username),
    UNIQUE KEY uq_internal_users_email    (email),
    KEY         idx_internal_users_role   (role_id),
    KEY         idx_internal_users_active (is_active),
    CONSTRAINT  fk_internal_users_role    FOREIGN KEY (role_id)     REFERENCES roles(id),
    CONSTRAINT  fk_internal_users_creator FOREIGN KEY (created_by)  REFERENCES internal_users(id)
        ON UPDATE CASCADE ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
