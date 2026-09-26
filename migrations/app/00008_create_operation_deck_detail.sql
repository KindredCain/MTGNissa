-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `operation_deck_detail` (
    `operation_id` BIGINT UNSIGNED NOT NULL,
    `deck_id` BIGINT UNSIGNED NOT NULL,
    `deck_name` VARCHAR(255) NOT NULL,
    PRIMARY KEY (`operation_id`),
    CONSTRAINT `fk_operation_deck_detail_log`
        FOREIGN KEY (`operation_id`) REFERENCES `operation_log` (`id`)
        ON UPDATE RESTRICT ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `operation_deck_detail`;
