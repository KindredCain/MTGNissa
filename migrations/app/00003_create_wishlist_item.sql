-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `wishlist_item` (
    `scryfall_id` CHAR(36) NOT NULL,
    `created_at` DATETIME(6) NOT NULL,
    PRIMARY KEY (`scryfall_id`),
    CONSTRAINT `fk_wishlist_item_print`
        FOREIGN KEY (`scryfall_id`) REFERENCES `card_print_ref` (`scryfall_id`)
        ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `wishlist_item`;
