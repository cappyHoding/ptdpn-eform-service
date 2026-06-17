CREATE TABLE `roles` (
  `id` tinyint unsigned NOT NULL AUTO_INCREMENT,
  `name` enum('admin','supervisor','operator') COLLATE utf8mb4_unicode_ci NOT NULL,
  `description` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_roles_name` (`name`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO `roles` (`id`, `name`, `description`) VALUES
    (1, 'admin',      'System administrator: manages users, config, and audit logs'),
    (2, 'supervisor', 'Checker: performs final approval of recommended applications'),
    (3, 'operator',   'Maker: performs initial review and recommends applications');

CREATE TABLE `internal_users` (
  `id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `username` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL,
  `full_name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `email` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `password` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'bcrypt hashed',
  `role_id` tinyint unsigned NOT NULL,
  `is_active` tinyint(1) NOT NULL DEFAULT '1',
  `created_by` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'internal_users.id of admin who created this account',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_internal_users_username` (`username`),
  UNIQUE KEY `uq_internal_users_email` (`email`),
  KEY `idx_internal_users_role` (`role_id`),
  KEY `idx_internal_users_active` (`is_active`),
  KEY `fk_internal_users_creator` (`created_by`),
  CONSTRAINT `fk_internal_users_creator` FOREIGN KEY (`created_by`) REFERENCES `internal_users` (`id`) ON DELETE SET NULL ON UPDATE CASCADE,
  CONSTRAINT `fk_internal_users_role` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `customers` (
  `id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `nik` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Populated after OCR Step 3',
  `full_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Populated after OCR Step 3',
  `mothers_maiden_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Filled in Step 4 - Additional Data',
  `current_address` text COLLATE utf8mb4_unicode_ci COMMENT 'Populated after OCR Step 3',
  `occupation` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Filled in Step 4 - Additional Data',
  `work_duration` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'e.g. "2 tahun 3 bulan"',
  `monthly_income` bigint unsigned DEFAULT NULL COMMENT 'Stored in IDR (Rupiah), no decimals',
  `education` enum('SD','SMP','SMA/SMK','D1','D2','D3','D4','S1','S2','S3','Lainnya') COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `email` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `phone_number` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `phone_number_wa` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'WhatsApp number, may differ from phone_number',
  `phone_verified` tinyint(1) NOT NULL DEFAULT '0' COMMENT '1 jika nomor HP sudah diverifikasi via OTP',
  `phone_verified_at` datetime DEFAULT NULL,
  `work_address` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_customers_nik` (`nik`),
  KEY `idx_customers_email` (`email`),
  KEY `idx_customers_deleted_at` (`deleted_at`),
  KEY `idx_customers_nik_active` (`nik`,`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `customer_sessions` (
  `id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `application_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `token` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'Signed JWT or opaque token, hashed before storage',
  `token_hash` char(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'SHA-256 of raw token for lookup',
  `ip_address` varchar(45) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Supports both IPv4 and IPv6',
  `user_agent` text COLLATE utf8mb4_unicode_ci,
  `expires_at` datetime NOT NULL,
  `revoked_at` datetime DEFAULT NULL COMMENT 'Set when customer completes step or session is invalidated',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_customer_sessions_token_hash` (`token_hash`),
  KEY `idx_customer_sessions_app_id` (`application_id`),
  KEY `idx_customer_sessions_expires` (`expires_at`),
  KEY `idx_sessions_token_expires` (`token_hash`,`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
