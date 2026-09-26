-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `operation_log` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `action_type` TINYINT UNSIGNED NOT NULL,
    `operation_date` DATE NOT NULL,
    `occurred_at` DATETIME(6) NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_operation_log_daily` (`operation_date`, `action_type`, `occurred_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `operation_log`;
