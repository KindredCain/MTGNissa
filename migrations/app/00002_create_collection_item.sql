-- +goose NO TRANSACTION

-- +goose Up
CREATE TABLE `collection_item` (
    `printing_uuid` CHAR(36) NOT NULL,
    `normal_quantity` INT UNSIGNED NOT NULL DEFAULT 0,
    `foil_quantity` INT UNSIGNED NOT NULL DEFAULT 0,
    `created_at` DATETIME(6) NOT NULL,
    `updated_at` DATETIME(6) NOT NULL,
    PRIMARY KEY (`printing_uuid`),
    CONSTRAINT `fk_collection_item_print`
        FOREIGN KEY (`printing_uuid`) REFERENCES `card_print_ref` (`printing_uuid`)
        ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE `collection_item`;
