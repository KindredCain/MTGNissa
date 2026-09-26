-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `deck_card` (
    `deck_id` BIGINT UNSIGNED NOT NULL,
    `section_type` VARCHAR(32) NOT NULL,
    `oracle_id` CHAR(36) NOT NULL,
    `quantity` INT UNSIGNED NOT NULL,
    `name_snapshot` VARCHAR(512) NOT NULL,
    PRIMARY KEY (`deck_id`, `section_type`, `oracle_id`),
    KEY `idx_deck_card_oracle` (`oracle_id`, `deck_id`, `quantity`),
    CONSTRAINT `fk_deck_card_deck`
        FOREIGN KEY (`deck_id`) REFERENCES `deck` (`id`)
        ON UPDATE RESTRICT ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `deck_card`;
