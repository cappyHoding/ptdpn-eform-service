CREATE TABLE `applications` (
  `id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `customer_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `product_type` enum('SAVING','DEPOSIT','LOAN') COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` enum('DRAFT','PENDING_REVIEW','IN_REVIEW','RECOMMENDED','APPROVED','REJECTED','FRAUD_REJECTED','SIGNING','COMPLETED','EXPIRED') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'DRAFT',
  `payment_proof_path` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Path file bukti transfer nasabah (SAVING/DEPOSIT only)',
  `payment_proof_at` datetime DEFAULT NULL,
  `current_step` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '1=Agreement, 2=Product, 3=OCR, 4=AddData, 5=Liveness, 6=Disbursement, 7=Summary, 8=Sign',
  `last_step_completed` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '0 means no step completed yet',
  `agreement_accepted` tinyint(1) NOT NULL DEFAULT '0',
  `agreement_accepted_at` datetime DEFAULT NULL,
  `agreement_ip` varchar(45) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `agreement_user_agent` text COLLATE utf8mb4_unicode_ci COMMENT 'Browser fingerprint for legal record',
  `submitted_at` datetime DEFAULT NULL,
  `rejection_reason` text COLLATE utf8mb4_unicode_ci COMMENT 'Set when status = REJECTED',
  `completed_at` datetime DEFAULT NULL COMMENT 'Set when status = COMPLETED',
  `sign_deadline` datetime DEFAULT NULL COMMENT 'Deadline for customer to sign after approval',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_applications_customer` (`customer_id`),
  KEY `idx_applications_status` (`status`),
  KEY `idx_applications_product` (`product_type`),
  KEY `idx_applications_created` (`created_at`),
  KEY `idx_applications_deleted` (`deleted_at`),
  KEY `idx_apps_status_product_created` (`status`,`product_type`,`created_at`),
  KEY `idx_apps_active_status` (`status`,`deleted_at`),
  KEY `idx_apps_signing_deadline` (`status`,`sign_deadline`),
  CONSTRAINT `fk_applications_customer` FOREIGN KEY (`customer_id`) REFERENCES `customers` (`id`) ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `saving_details` (
  `application_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `product_name` enum('Tabungan Perdana','Tabungan Perdana Plus','TabunganKu') COLLATE utf8mb4_unicode_ci NOT NULL,
  `initial_deposit` bigint unsigned NOT NULL COMMENT 'In IDR',
  `source_of_funds` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `saving_purpose` varchar(200) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'e.g. Tabungan Rutin, Dana Darurat',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`application_id`),
  CONSTRAINT `fk_saving_details_application` FOREIGN KEY (`application_id`) REFERENCES `applications` (`id`) ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `deposit_details` (
  `application_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `product_name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'Deposito Perdana',
  `placement_amount` bigint unsigned NOT NULL COMMENT 'In IDR',
  `tenor_months` tinyint unsigned NOT NULL COMMENT 'Must be 1, 3, 6, or 12',
  `rollover_type` enum('ARO','NON_ARO','ARO_RATE') COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'ARO = Automatic Roll Over, NON_ARO = Tidak Diperpanjang',
  `interest_rate` decimal(5,2) DEFAULT NULL COMMENT 'Snapshot of rate at time of application from system_config',
  `source_of_funds` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `investment_purpose` varchar(200) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`application_id`),
  CONSTRAINT `fk_deposit_details_application` FOREIGN KEY (`application_id`) REFERENCES `applications` (`id`) ON UPDATE CASCADE,
  CONSTRAINT `chk_deposit_tenor` CHECK ((`tenor_months` in (1,3,6,12)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `loan_details` (
  `application_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `product_name` enum('Kredit Modal Kerja','Kredit Aneka Guna') COLLATE utf8mb4_unicode_ci NOT NULL,
  `requested_amount` bigint unsigned NOT NULL COMMENT 'In IDR',
  `tenor_months` tinyint unsigned NOT NULL COMMENT 'Loan repayment period in months',
  `interest_rate` decimal(5,2) DEFAULT NULL COMMENT 'Snapshot of rate from system_config at application time',
  `loan_purpose` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'Applicant description of how funds will be used',
  `payment_source` varchar(200) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'How the applicant plans to repay, e.g. salary',
  `source_of_funds` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`application_id`),
  CONSTRAINT `fk_loan_details_application` FOREIGN KEY (`application_id`) REFERENCES `applications` (`id`) ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `collateral_items` (
  `id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `application_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `collateral_type` enum('SHM','SHGB','BPKB','Deposito','Lainnya') COLLATE utf8mb4_unicode_ci NOT NULL,
  `description` text COLLATE utf8mb4_unicode_ci COMMENT 'e.g. "Toyota Avanza 2019, Plat B 1234 XY"',
  `estimated_value` bigint unsigned DEFAULT NULL COMMENT 'In IDR, assessed by applicant',
  `attachment_path` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'Local path: /var/app/storage/collateral/{year}/{month}/{filename}',
  `sort_order` tinyint unsigned NOT NULL DEFAULT '1' COMMENT 'Display order in admin UI',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_collateral_application` (`application_id`),
  CONSTRAINT `fk_collateral_application` FOREIGN KEY (`application_id`) REFERENCES `applications` (`id`) ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `disbursement_data` (
  `id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `application_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `bank_name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'Human-readable bank name, e.g. Bank Central Asia',
  `bank_code` varchar(10) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'BI standard routing code, e.g. 014 for BCA',
  `account_number` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL,
  `account_holder` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'Name as printed on the bank account',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_disbursement_application` (`application_id`),
  CONSTRAINT `fk_disbursement_application` FOREIGN KEY (`application_id`) REFERENCES `applications` (`id`) ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `customer_sessions` 
  ADD CONSTRAINT `fk_customer_sessions_application` FOREIGN KEY (`application_id`) REFERENCES `applications` (`id`) ON UPDATE CASCADE;
