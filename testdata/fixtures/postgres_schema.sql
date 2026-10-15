-- Dataprobe Dummy/Fixture Database Schema (PostgreSQL 12+)
-- For local testing, manual verification, and CI test harnesses

CREATE SCHEMA IF NOT EXISTS store_schema;
SET search_path TO store_schema, public;

DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS raw_events;
DROP TABLE IF EXISTS accounts;

-- 1. Accounts Table (Primary key and unique index)
CREATE TABLE accounts (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 2. Orders Table (Foreign key with CASCADE)
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    amount NUMERIC(10,2) NOT NULL DEFAULT 0.00,
    status VARCHAR(50) NOT NULL DEFAULT 'new'
);
CREATE INDEX idx_orders_account_id ON orders(account_id);

-- 3. Order Items Table
CREATE TABLE order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sku VARCHAR(64) NOT NULL,
    qty INT NOT NULL DEFAULT 1
);

-- 4. Raw Events Table (No primary key for SCH_001 audit rule testing)
CREATE TABLE raw_events (
    event_name VARCHAR(100) NOT NULL,
    payload JSONB,
    received_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Seed Dummy Records
INSERT INTO accounts (username, email) VALUES 
('alex', 'alex@example.com'),
('sarah', 'sarah@example.com');

INSERT INTO orders (account_id, amount, status) VALUES 
(1, 99.50, 'paid'),
(2, 250.00, 'pending');

INSERT INTO order_items (order_id, sku, qty) VALUES 
(1, 'SKU-001', 2),
(2, 'SKU-999', 1);

INSERT INTO raw_events (event_name, payload) VALUES 
('user.login', '{"ip": "127.0.0.1"}');
