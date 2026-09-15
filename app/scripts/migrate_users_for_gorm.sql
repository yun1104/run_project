-- 修复旧版 users 表（含 password 列）以兼容 GORM UserAccount（使用 password_hash）
USE meituan_db_0;

-- 1. 若有 password 列则改为可空，避免 INSERT 时报 "Field 'password' doesn't have a default value"
ALTER TABLE users MODIFY COLUMN password VARCHAR(100) NULL DEFAULT NULL;
