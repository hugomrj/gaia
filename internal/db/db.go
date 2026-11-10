package db

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// Create crea la DB en path y ejecuta el schema.
// Falla si el archivo ya existe (para no pisar datos por accidente).
func Create(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("la DB ya existe en %s (borrala si querés recrearla)", path)
	}

	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(schemaTenants); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	return nil
}

// Open abre una DB existente. Falla si el archivo no existe.
func Open(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("la DB no existe en %s (corré cmd/db-init primero)", path)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}

const schemaTenants = `
CREATE TABLE IF NOT EXISTS tenants (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    provider        TEXT NOT NULL,
    api_key_plain   TEXT,
    api_key_enc     BLOB,
    token_hash      TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'active',
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tenants_slug ON tenants(slug);
CREATE INDEX IF NOT EXISTS idx_tenants_token_hash ON tenants(token_hash);
CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);
`