CREATE DATABASE IF NOT EXISTS meituan_db_0 DEFAULT CHARSET utf8mb4;
CREATE DATABASE IF NOT EXISTS meituan_db_1 DEFAULT CHARSET utf8mb4;

USE meituan_db_0;

CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(50) UNIQUE NOT NULL,
    password VARCHAR(100) NOT NULL,
    phone VARCHAR(20) UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_username (username),
    INDEX idx_phone (phone)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_preferences_0 (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    categories TEXT,
    price_range VARCHAR(50),
    tastes TEXT,
    merchants TEXT,
    dish_keywords TEXT,
    avoid_foods TEXT,
    order_times TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS orders_202601 (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    merchant_id BIGINT,
    merchant_name VARCHAR(100),
    dishes TEXT,
    total_price DECIMAL(10,2),
    status INT DEFAULT 0,
    order_time TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_merchant_id (merchant_id),
    INDEX idx_order_time (order_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
