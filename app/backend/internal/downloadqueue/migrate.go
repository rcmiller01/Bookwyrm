package downloadqueue

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.up.sql
var migrationFS embed.FS

type migrationFile struct {
	version int
	name    string
	sql     string
}

func EmbeddedMigrationVersions() []int {
	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		return []int{}
	}
	versions := make([]int, 0, len(migrations))
	for _, migration := range migrations {
		versions = append(versions, migration.version)
	}
	return versions
}

func LatestEmbeddedMigrationVersion() int {
	versions := EmbeddedMigrationVersions()
	if len(versions) == 0 {
		return 0
	}
	return versions[len(versions)-1]
}

func RunMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
		  version INT PRIMARY KEY,
		  name TEXT NOT NULL,
		  applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		var applied bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, migration.version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %d: %w", migration.version, err)
		}
		if applied {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", migration.version, err)
		}
		if _, err := tx.ExecContext(ctx, migration.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", migration.version, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, applied_at) VALUES ($1, $2, NOW())`, migration.version, migration.name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", migration.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.version, err)
		}
	}
	return nil
}

func loadEmbeddedMigrations() ([]migrationFile, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	migrations := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		version, err := parseVersion(entry.Name())
		if err != nil {
			return nil, err
		}
		content, err := migrationFS.ReadFile(filepath.ToSlash(filepath.Join("migrations", entry.Name())))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		migrations = append(migrations, migrationFile{
			version: version,
			name:    entry.Name(),
			sql:     string(content),
		})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	return migrations, nil
}

func parseVersion(name string) (int, error) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid migration filename %s", name)
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("parse migration version %s: %w", name, err)
	}
	return version, nil
}
