-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `collection_item` (
    `scryfall_id` CHAR(36) NOT NULL,
    `normal_quantity` INT UNSIGNED NOT NULL DEFAULT 0,
    `foil_quantity` INT UNSIGNED NOT NULL DEFAULT 0,
    `created_at` DATETIME(6) NOT NULL,
    `updated_at` DATETIME(6) NOT NULL,
    PRIMARY KEY (`scryfall_id`),
    CONSTRAINT `fk_collection_item_print`
        FOREIGN KEY (`scryfall_id`) REFERENCES `card_print_ref` (`scryfall_id`)
        ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `collection_item`;
