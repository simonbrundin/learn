package store

import (
	"fmt"
	"os"
	"strings"
)

// OpenFromEnv öppnar det lager som miljön pekar ut.
// DATABASE_URL väljer PostgreSQL, annars används SQLite med sökvägen från
// LEARN_DB (eller standardplatsen).
func OpenFromEnv() (Repository, error) {
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		repo, err := OpenPostgres(dsn)
		if err != nil {
			return nil, err
		}
		return repo, nil
	}
	repo, err := Open(DefaultPath())
	if err != nil {
		return nil, fmt.Errorf("kunde inte öppna kunskapsbasen: %w", err)
	}
	return repo, nil
}
