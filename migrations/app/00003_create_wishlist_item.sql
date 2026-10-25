-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `wishlist_item` (
    `printing_uuid` CHAR(36) NOT NULL,
    `created_at` DATETIME(6) NOT NULL,
    PRIMARY KEY (`printing_uuid`),
    CONSTRAINT `fk_wishlist_item_print`
        FOREIGN KEY (`printing_uuid`) REFERENCES `card_print_ref` (`printing_uuid`)
        ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `wishlist_item`;
