-- Dataprobe Dummy/Fixture Database Schema (MySQL 8.x / 5.7)
-- For local testing, manual verification, and CI test harnesses

CREATE DATABASE IF NOT EXISTS shop_db;
USE shop_db;

-- 1. Users Table (Clean base table with Primary Key and Unique Index)
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS unindexed_logs;
DROP TABLE IF EXISTS duplicate_index_demo;
DROP TABLE IF EXISTS users;

CREATE TABLE users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_users_email UNIQUE (email)
) ENGINE=InnoDB;

-- 2. Orders Table (Foreign key referencing users)
CREATE TABLE orders (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    total_amount DECIMAL(12,2) NOT NULL DEFAULT 0.00,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_orders_user_id (user_id),
    CONSTRAINT fk_orders_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
) ENGINE=InnoDB;

-- 3. Order Items Table (Foreign key with CASCADE)
CREATE TABLE order_items (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    order_id BIGINT NOT NULL,
    product_name VARCHAR(255) NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    unit_price DECIMAL(10,2) NOT NULL DEFAULT 0.00,
    INDEX idx_order_items_order_id (order_id),
    CONSTRAINT fk_order_items_order_id FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- 4. Unindexed Logs Table (Intentional structural issue: NO PRIMARY KEY for SCH_001 audit rule testing)
CREATE TABLE unindexed_logs (
    event_type VARCHAR(64) NOT NULL,
    payload TEXT,
    created_at DATETIME NOT NULL
) ENGINE=InnoDB;

-- 5. Duplicate Index Demo Table (Intentional structural issue: duplicate indexes for IDX_001 audit rule testing)
CREATE TABLE duplicate_index_demo (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    category_id INT NOT NULL,
    tag_id INT NOT NULL,
    INDEX idx_cat_tag_1 (category_id, tag_id),
    INDEX idx_cat_tag_2 (category_id, tag_id)
) ENGINE=InnoDB;

-- Seed Dummy Records
INSERT INTO users (email, full_name) VALUES 
('alice@example.com', 'Alice Smith'),
('bob@example.com', 'Bob Jones'),
('charlie@example.com', 'Charlie Brown');

INSERT INTO orders (user_id, total_amount, status) VALUES 
(1, 150.00, 'completed'),
(1, 45.50, 'completed'),
(2, 320.00, 'pending');

INSERT INTO order_items (order_id, product_name, quantity, unit_price) VALUES 
(1, 'Mechanical Keyboard', 1, 120.00),
(1, 'Mousepad', 1, 30.00),
(2, 'USB-C Cable', 2, 22.75),
(3, 'Wireless Headphones', 1, 320.00);

INSERT INTO unindexed_logs (event_type, payload, created_at) VALUES 
('order.created', '{"order_id": 1}', NOW()),
('payment.received', '{"order_id": 1}', NOW());
