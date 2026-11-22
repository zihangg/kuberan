package database

import (
	"os"
	"regexp"
	"testing"

	"github.com/golang-migrate/migrate/v4/source"
	_ "github.com/golang-migrate/migrate/v4/source/file" // registers the file:// source
)

// migrationsDir is the migrations folder relative to this package.
const migrationsDir = "../../migrations"

var migrationFileRe = regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.sql$`)

// TestMigrationFiles_OpenLikeTheMigrator opens the folder the way the migrate
// CLI and the API do at startup. golang-migrate refuses a folder in which two
// files share a version (for example two 000058_*.up.sql from branches merged
// in parallel), which would otherwise only surface at deploy time.
func TestMigrationFiles_OpenLikeTheMigrator(t *testing.T) {
	src, err := source.Open("file://" + migrationsDir)
	if err != nil {
		t.Fatalf("opening migrations as the migrator does: %v", err)
	}
	if err := src.Close(); err != nil {
		t.Fatalf("closing migrations source: %v", err)
	}
}

// TestMigrationFiles_OneUpAndDownPerVersion checks every version has exactly
// one up and one down file, with the same name.
func TestMigrationFiles_OneUpAndDownPerVersion(t *testing.T) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("reading %s: %v", migrationsDir, err)
	}
	type pair struct{ up, down []string }
	byVersion := map[string]*pair{}
	for _, e := range entries {
		m := migrationFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		p := byVersion[m[1]]
		if p == nil {
			p = &pair{}
			byVersion[m[1]] = p
		}
		if m[3] == "up" {
			p.up = append(p.up, m[2])
		} else {
			p.down = append(p.down, m[2])
		}
	}
	if len(byVersion) == 0 {
		t.Fatalf("no migration files found in %s", migrationsDir)
	}
	for v, p := range byVersion {
		if len(p.up) != 1 || len(p.down) != 1 {
			t.Errorf("version %s: want one up and one down file, got up=%v down=%v", v, p.up, p.down)
			continue
		}
		if p.up[0] != p.down[0] {
			t.Errorf("version %s: up %q and down %q have different names", v, p.up[0], p.down[0])
		}
	}
}
