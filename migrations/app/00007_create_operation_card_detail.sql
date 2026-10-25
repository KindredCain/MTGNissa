-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `operation_card_detail` (
    `operation_id` BIGINT UNSIGNED NOT NULL,
    `printing_uuid` CHAR(36) NOT NULL,
    `oracle_id` CHAR(36) NULL,
    `card_name` VARCHAR(512) NOT NULL,
    `set_code` VARCHAR(16) NOT NULL,
    `collector_number` VARCHAR(64) NOT NULL,
    `lang` VARCHAR(8) NOT NULL,
    `lang_display_name` VARCHAR(64) NOT NULL,
    `front_url` TEXT NULL,
    `back_url` TEXT NULL,
    `has_distinct_back` BOOLEAN NOT NULL,
    `finish_kind` TINYINT UNSIGNED NULL,
    `quantity_delta` BIGINT NULL,
    PRIMARY KEY (`operation_id`),
    CONSTRAINT `fk_operation_card_detail_log`
        FOREIGN KEY (`operation_id`) REFERENCES `operation_log` (`id`)
        ON UPDATE RESTRICT ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `operation_card_detail`;
