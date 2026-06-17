-- =============================================================
-- Migration 006: Add eSign TOS acceptance fields
-- BPR Perdana E-Form System
-- =============================================================
-- Menambah kolom untuk menyimpan bukti nasabah menyetujui
-- Syarat & Ketentuan eSign VIDA sebelum diarahkan ke link TTD.

ALTER TABLE `contract_documents`
  ADD COLUMN `esign_tos_accepted`    tinyint(1)   NOT NULL DEFAULT 0
    COMMENT '1 = nasabah sudah centang checkbox agreement eSign VIDA'
    AFTER `signed_file_path`,
  ADD COLUMN `esign_tos_accepted_at` datetime     DEFAULT NULL
    COMMENT 'Timestamp ketika nasabah menyetujui TOS eSign'
    AFTER `esign_tos_accepted`,
  ADD COLUMN `esign_tos_ip`          varchar(45)  COLLATE utf8mb4_unicode_ci DEFAULT NULL
    COMMENT 'IP address nasabah saat menyetujui TOS (audit trail)'
    AFTER `esign_tos_accepted_at`;
