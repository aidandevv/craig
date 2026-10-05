// Package cache is the only SQLite owner in the daemon. It stores two things:
// results of billable image signals, keyed by image hash, and a durable ledger
// of API units spent this month.
//
// Nothing about a listing is persisted. Analysis is transient by design.
package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aidandevv/craig/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

// Open prepares the cache at path, creating parent directories as needed.
// Pass ":memory:" for tests.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("cache path is required")
	}
	if path != ":memory:" {
		path = filepath.Clean(path)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create cache directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// A single connection keeps SQLite writes serialized without lock churn;
	// the daemon's request volume is a handful per minute.
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize cache schema: %w", err)
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Get returns a cached signal result. The boolean reports whether one existed.
func (s *Store) Get(ctx context.Context, imageHash, signal string) (domain.SignalResult, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx,
		`SELECT result FROM image_signal_cache WHERE image_hash=? AND signal=?`,
		imageHash, signal).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SignalResult{}, false, nil
	}
	if err != nil {
		return domain.SignalResult{}, false, err
	}
	var result domain.SignalResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return domain.SignalResult{}, false, err
	}
	return result, true, nil
}

// Put stores a signal result. Skipped results are never cached: a signal that
// could not run must be retried next time, not remembered as an answer.
func (s *Store) Put(ctx context.Context, imageHash, signal string, result domain.SignalResult) error {
	if result.Skipped != "" {
		return nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO image_signal_cache(image_hash,signal,result) VALUES(?,?,?)`,
		imageHash, signal, string(raw))
	return err
}

// ReserveVisionUnit atomically records one unit if this feature is still below
// its monthly cap. Vision bills each feature independently, so each gets its own
// ledger line. The ledger, not an in-memory counter, is the durable budget:
// restarting the daemon must not hand the user a fresh allowance.
func (s *Store) ReserveVisionUnit(ctx context.Context, feature string, monthlyCap int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var used int
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(unit_count),0) FROM vision_ledger
		 WHERE feature=? AND date >= date('now','start of month')`, feature).Scan(&used)
	if err != nil {
		return false, err
	}
	if used >= monthlyCap {
		return false, nil
	}
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO vision_ledger(date,feature,unit_count) VALUES(date('now'),?,1)`,
		feature); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// UnitsUsed reports units spent on a feature this month, for display.
func (s *Store) UnitsUsed(ctx context.Context, feature string) (int, error) {
	var used int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(unit_count),0) FROM vision_ledger
		 WHERE feature=? AND date >= date('now','start of month')`, feature).Scan(&used)
	return used, err
}

// Prune drops cached results older than maxAge. Reverse-image findings go stale;
// a photo absent from the web today may be all over it next month.
func (s *Store) Prune(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-maxAge)
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM image_signal_cache WHERE created_at < ?`,
		cutoff.Format("2006-01-02 15:04:05"))
	if err != nil {
		return 0, err
	}
	legacy, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	evidence, err := s.db.ExecContext(ctx, `DELETE FROM vision_evidence_v2 WHERE checked_at < ?`, time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339Nano))
	if err != nil {
		return legacy, err
	}
	count, err := evidence.RowsAffected()
	return legacy + count, err
}

// Provider evidence is independent of rule scoring. Legacy scored negatives
// are deliberately never read by this cache. Expiration is enforced on reads.
func (s *Store) GetEvidence(ctx context.Context, hash, feature string) (map[string]any, time.Time, bool) {
	var raw, checked string
	err := s.db.QueryRowContext(ctx, `SELECT response, checked_at FROM vision_evidence_v2 WHERE image_hash=? AND feature=?`, hash, feature).Scan(&raw, &checked)
	at, parseErr := time.Parse(time.RFC3339Nano, checked)
	if err != nil || parseErr != nil || time.Since(at) > 24*time.Hour {
		return nil, time.Time{}, false
	}
	var data map[string]any
	if json.Unmarshal([]byte(raw), &data) != nil {
		return nil, time.Time{}, false
	}
	return data, at, true
}

func (s *Store) PutEvidence(ctx context.Context, hash, feature string, data map[string]any, at time.Time) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT OR REPLACE INTO vision_evidence_v2(image_hash,feature,response,checked_at) VALUES(?,?,?,?)`, hash, feature, string(raw), at.UTC().Format(time.RFC3339Nano))
	return err
}
