-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `operation_log` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `action_type` TINYINT UNSIGNED NOT NULL,
    `occurred_at` DATETIME(6) NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_operation_log_occurred_action` (`occurred_at`, `action_type`, `id`),
    KEY `idx_operation_log_action_occurred` (`action_type`, `occurred_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `operation_log`;
