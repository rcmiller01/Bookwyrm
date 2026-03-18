package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"metadata-service/migrations"
)

type migrationFile struct {
	version int
	name    string
	sql     string
}

const migrationTable = "metadata_schema_migrations"

func RunMigrations(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
		  version INT PRIMARY KEY,
		  name TEXT NOT NULL,
		  applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`, migrationTable)); err != nil {
		return fmt.Errorf("create %s: %w", migrationTable, err)
	}

	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	files := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		version, err := parseMigrationVersion(entry.Name())
		if err != nil {
			return err
		}
		content, err := migrations.Files.ReadFile(filepath.ToSlash(entry.Name()))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		files = append(files, migrationFile{
			version: version,
			name:    entry.Name(),
			sql:     string(content),
		})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })

	for _, migration := range files {
		var applied bool
		if err := db.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE version=$1)`, migrationTable), migration.version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %d: %w", migration.version, err)
		}
		if applied {
			continue
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", migration.version, err)
		}

		if _, err := tx.Exec(ctx, migration.sql); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %d: %w", migration.version, err)
		}

		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (version, name, applied_at) VALUES ($1, $2, NOW())`, migrationTable), migration.version, migration.name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %d: %w", migration.version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.version, err)
		}
	}

	return nil
}

func parseMigrationVersion(filename string) (int, error) {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid migration filename %s", filename)
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid migration version %s: %w", filename, err)
	}
	return version, nil
}

