package migrations

import (
	"sort"
	"strings"
)

// ListSQLFiles returns embedded migration filenames in sorted order.
func ListSQLFiles() ([]string, error) {
	entries, err := FS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

// FilesForPostgres skips SQLite-only *_mono.sql migrations.
func FilesForPostgres(all []string) []string {
	var out []string
	for _, f := range all {
		if strings.HasSuffix(f, "_mono.sql") {
			continue
		}
		out = append(out, f)
	}
	return out
}

// postgresOnlyMigrations are embedded for Unified/Postgres and must not run on SQLite.
var postgresOnlyMigrations = map[string]bool{
	"005_partition_emails.sql":             true,
	"006_uid_validity_bigint.sql":          true, // ALTER COLUMN ... TYPE — Postgres-only
	"010_partition_emails_64.sql":          true,
	"021_folders_name_lower_generated.sql": true, // DO $$ / GENERATED ALWAYS
}

// IsPostgresOnlyMigration reports whether a migration file is Postgres-specific.
func IsPostgresOnlyMigration(filename string) bool {
	return postgresOnlyMigrations[filename]
}

// FilesForSQLite prefers *_mono.sql when a numbered pair exists (e.g. 015 + 015_mono).
func FilesForSQLite(all []string) []string {
	hasMonoTwin := make(map[string]bool)
	for _, f := range all {
		if strings.HasSuffix(f, "_mono.sql") {
			base := strings.TrimSuffix(f, "_mono.sql") + ".sql"
			hasMonoTwin[base] = true
		}
	}
	var out []string
	for _, f := range all {
		if IsPostgresOnlyMigration(f) {
			continue
		}
		if strings.HasSuffix(f, "_mono.sql") {
			out = append(out, f)
			continue
		}
		if hasMonoTwin[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}
