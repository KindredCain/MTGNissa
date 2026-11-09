-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `card_print_ref` (
    `printing_uuid` CHAR(36) NOT NULL,
    `oracle_id` CHAR(36) NULL,
    `set_code` VARCHAR(16) NOT NULL,
    `collector_number` VARCHAR(64) NOT NULL,
    `collector_sort_key` VARBINARY(512) NOT NULL,
    `lang` VARCHAR(8) NOT NULL,
    `name` VARCHAR(512) NOT NULL,
    `colors_mask` BIGINT UNSIGNED NOT NULL,
    `cmc` DECIMAL(12,4) NOT NULL,
    `rarity` VARCHAR(32) NOT NULL,
    `released_at` DATE NULL,
    `set_released_at` DATE NULL,
    `is_real` BOOLEAN NOT NULL DEFAULT FALSE,
    `created_at` DATETIME(6) NOT NULL,
    `last_verified_at` DATETIME(6) NOT NULL,
    PRIMARY KEY (`printing_uuid`),
    KEY `idx_card_print_ref_oracle` (`oracle_id`),
    KEY `idx_card_print_ref_set_print` (`set_code`, `lang`, `collector_number`),
    KEY `idx_card_print_ref_lang_oracle` (`lang`, `oracle_id`),
    KEY `idx_card_print_ref_real_print` (`is_real`, `printing_uuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `card_print_ref`;
