-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `card_print_ref` (
    `printing_uuid` CHAR(36) NOT NULL,
    `oracle_id` CHAR(36) NULL,
    `set_code` VARCHAR(16) NOT NULL,
    `collector_number` VARCHAR(64) NOT NULL,
    `lang` VARCHAR(8) NOT NULL,
    `created_at` DATETIME(6) NOT NULL,
    `last_verified_at` DATETIME(6) NOT NULL,
    PRIMARY KEY (`printing_uuid`),
    KEY `idx_card_print_ref_oracle` (`oracle_id`),
    KEY `idx_card_print_ref_set_print` (`set_code`, `lang`, `collector_number`),
    KEY `idx_card_print_ref_lang_oracle` (`lang`, `oracle_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `card_print_ref`;
