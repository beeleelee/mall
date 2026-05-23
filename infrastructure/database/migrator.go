package database

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Migration struct {
	Version int
	UpSQL   string
	DownSQL string
}

type Migrator struct {
	db *sqlx.DB
}

func NewMigrator(db *sqlx.DB) *Migrator {
	return &Migrator{db: db}
}

func (m *Migrator) Up() error {
	if err := m.ensureTable(); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return fmt.Errorf("get applied versions: %w", err)
	}

	for _, mig := range migrations {
		if applied[mig.Version] {
			continue
		}
		if err := m.apply(mig); err != nil {
			return fmt.Errorf("apply migration %d: %w", mig.Version, err)
		}
	}

	return nil
}

func (m *Migrator) ensureTable() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func (m *Migrator) loadMigrations() ([]Migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	type filePair struct {
		version int
		up      string
		down    string
		name    string
	}

	pairs := map[int]*filePair{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		var version int
		var kind string
		if _, err := fmt.Sscanf(name, "%d_%s", &version, &kind); err != nil {
			continue
		}

		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, err
		}

		if _, ok := pairs[version]; !ok {
			pairs[version] = &filePair{version: version, name: name}
		}

		if strings.HasSuffix(name, ".up.sql") {
			pairs[version].up = string(data)
		} else if strings.HasSuffix(name, ".down.sql") {
			pairs[version].down = string(data)
		}
	}

	versions := make([]int, 0, len(pairs))
	for v := range pairs {
		versions = append(versions, v)
	}
	sort.Ints(versions)

	migrations := make([]Migration, 0, len(versions))
	for _, v := range versions {
		migrations = append(migrations, Migration{
			Version: v,
			UpSQL:   pairs[v].up,
			DownSQL: pairs[v].down,
		})
	}

	return migrations, nil
}

func (m *Migrator) appliedVersions() (map[int]bool, error) {
	rows, err := m.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func (m *Migrator) apply(mig Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(mig.UpSQL); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", mig.Version); err != nil {
		return err
	}

	return tx.Commit()
}
