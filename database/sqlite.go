package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func InitDB(dbPath string) (*sql.DB, error) {
	// _pragma=journal_mode(WAL) 提升并发性能；_pragma=busy_timeout(5000) 避免锁冲突
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite 写操作单连接最稳妥

	// 自动建表（以后要加 WebAuthn，直接在这里追加一张表即可）
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'viewer',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS devices (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		uuid TEXT UNIQUE NOT NULL,
		type TEXT NOT NULL,
		id_adb TEXT NOT NULL,
		display_name TEXT UNIQUE NOT NULL,
		ip TEXT,
		port INTEGER,
		status_adb TEXT NOT NULL,
		status_ui TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS share_links (
		token TEXT PRIMARY KEY,
		screen_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		is_revoked BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
