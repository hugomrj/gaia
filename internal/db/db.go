//  internal/db/db.go
package db

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

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
    token_hash      TEXT NOT NULL UNIQUE,
    token_plain     TEXT NOT NULL, -- <-- NUEVA COLUMNA
    status          TEXT NOT NULL DEFAULT 'active',
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS providers (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE
);

INSERT OR IGNORE INTO providers (name) VALUES ('mistral'), ('openai'), ('anthropic');

CREATE TABLE IF NOT EXISTS tenant_proveedor (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id       INTEGER NOT NULL,
    provider_id     INTEGER NOT NULL,
    is_default      INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (provider_id) REFERENCES providers(id),
    UNIQUE(tenant_id, provider_id)
);

CREATE TABLE IF NOT EXISTS keys (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_proveedor_id INTEGER NOT NULL,
    api_key_plain       TEXT,
    api_key_enc         BLOB,
    is_active           INTEGER NOT NULL DEFAULT 1,
    usage_count         INTEGER NOT NULL DEFAULT 0, -- <-- ESTO ES CLAVE
    created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_proveedor_id) REFERENCES tenant_proveedor(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tenants_slug ON tenants(slug);
CREATE INDEX IF NOT EXISTS idx_tenants_token_hash ON tenants(token_hash);
CREATE INDEX IF NOT EXISTS idx_tp_tenant ON tenant_proveedor(tenant_id);
CREATE INDEX IF NOT EXISTS idx_keys_tp ON keys(tenant_proveedor_id);
`