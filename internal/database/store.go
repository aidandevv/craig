// Package database is the sole SQLite owner. No other process accesses the
// database file; UI and sidecars use the Go HTTP API instead.
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(databaseURL string) (*Store, error) {
	path := strings.TrimPrefix(databaseURL, "sqlite:///")
	if path == databaseURL {
		return nil, fmt.Errorf("only sqlite:/// URLs are supported")
	}
	if path != ":memory:" {
		path = filepath.Clean(path)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.Initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Initialize(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *Store) InsertDiscovered(ctx context.Context, l domain.Listing) (int64, bool, error) {
	if l.SourceID == "" {
		return 0, false, errors.New("listing source ID is required")
	}
	if strings.TrimSpace(l.Title) == "" {
		l.Title = "Untitled listing"
	}
	res, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO listings (source_id,title,price,bedrooms,bathrooms,location,posted_at,craigslist_url,seen_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, l.SourceID, l.Title, l.Price, l.Bedrooms, l.Bathrooms, l.Location, nullableTime(l.PostedAt), l.URL)
	if err != nil {
		return 0, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, false, err
	}
	if n == 0 {
		var id int64
		err = s.db.QueryRowContext(ctx, `SELECT id FROM listings WHERE source_id=?`, l.SourceID).Scan(&id)
		return id, false, err
	}
	id, err := res.LastInsertId()
	return id, true, err
}

func (s *Store) ApplyEnrichment(ctx context.Context, id int64, l domain.Listing) error {
	images, err := json.Marshal(l.Images)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE listings SET title=?, price=?, bedrooms=?, bathrooms=?, location=?, description=?, contact_email=?, contact_phone=?, posted_at=?, craigslist_url=?, images=?, image_count=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		fallback(l.Title, "Untitled listing"), l.Price, l.Bedrooms, l.Bathrooms, l.Location, l.Description, l.ContactEmail, l.ContactPhone, nullableTime(l.PostedAt), l.URL, string(images), len(l.Images), id)
	return err
}

func (s *Store) SaveEvaluation(ctx context.Context, id int64, result domain.FraudResult) error {
	flags, err := json.Marshal(result.Flags)
	if err != nil {
		return err
	}
	matches := []string{}
	for _, signal := range result.Signals {
		if signal.Name == "reverse_image" {
			matches = append(matches, signal.Details...)
		}
	}
	matchesJSON, err := json.Marshal(matches)
	if err != nil {
		return err
	}
	var source string
	for _, f := range result.Flags {
		if strings.HasPrefix(f, "reverse_match_") {
			source = strings.TrimPrefix(f, "reverse_match_")
			break
		}
	}
	_, err = s.db.ExecContext(ctx, `UPDATE listings SET fraud_score=?, fraud_flags=?, fit_score=?, urgent_eligible=?, fraud_evaluation_time=?, reverse_image_matches=?, reverse_image_source=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, result.Score, string(flags), result.FitScore, result.Urgent, result.ElapsedMS, string(matchesJSON), source, id)
	return err
}

func (s *Store) Get(ctx context.Context, id int64) (domain.Listing, error) {
	return scanListing(s.db.QueryRowContext(ctx, listingSelect+` WHERE id=?`, id))
}

func (s *Store) ListQualified(ctx context.Context, maxFraud float64, page, perPage int) ([]domain.Listing, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM listings WHERE fraud_score IS NOT NULL AND fraud_score <= ?`, maxFraud).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, listingSelect+` WHERE fraud_score IS NOT NULL AND fraud_score <= ? ORDER BY seen_at DESC LIMIT ? OFFSET ?`, maxFraud, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	listings := make([]domain.Listing, 0)
	for rows.Next() {
		l, err := scanListing(rows)
		if err != nil {
			return nil, 0, err
		}
		listings = append(listings, l)
	}
	return listings, total, rows.Err()
}

func (s *Store) Mark(ctx context.Context, id int64, status, notes string) error {
	valid := map[string]bool{"new": true, "interested": true, "contacted": true, "toured": true, "applied": true, "passed": true}
	if !valid[status] {
		return fmt.Errorf("invalid listing status %q", status)
	}
	var stamp string
	switch status {
	case "contacted":
		stamp = "contacted_at"
	case "toured":
		stamp = "toured_at"
	case "applied":
		stamp = "applied_at"
	}
	query := `UPDATE listings SET status=?, user_notes=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`
	if stamp != "" {
		query = `UPDATE listings SET status=?, user_notes=?, ` + stamp + `=COALESCE(` + stamp + `,CURRENT_TIMESTAMP), updated_at=CURRENT_TIMESTAMP WHERE id=?`
	}
	result, err := s.db.ExecContext(ctx, query, status, notes, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) Stats(ctx context.Context, maxFraud float64) (domain.Stats, error) {
	var out domain.Stats
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN fraud_score IS NOT NULL AND fraud_score <= ? THEN 1 ELSE 0 END),0), COALESCE(AVG(fraud_score),0) FROM listings WHERE date(seen_at)=date('now')`, maxFraud).Scan(&out.SeenToday, &out.QualifiedToday, &out.AverageFraud)
	if err != nil {
		return out, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(cost),0) FROM cost_ledger WHERE date=date('now')`).Scan(&out.CostToday)
	if err != nil {
		return out, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM listings`).Scan(&out.TotalListings)
	return out, err
}

func (s *Store) GetCache(ctx context.Context, imageHash, signal string) (domain.SignalResult, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT result FROM image_signal_cache WHERE image_hash=? AND signal=?`, imageHash, signal).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SignalResult{}, false, nil
	}
	if err != nil {
		return domain.SignalResult{}, false, err
	}
	var result domain.SignalResult
	err = json.Unmarshal([]byte(raw), &result)
	return result, err == nil, err
}

func (s *Store) PutCache(ctx context.Context, imageHash, signal string, result domain.SignalResult) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO image_signal_cache(image_hash,signal,result) VALUES(?,?,?)`, imageHash, signal, string(raw))
	return err
}

// ReserveVisionUnit atomically records one free-tier unit only if this feature
// remains below its independent monthly cap. The ledger is the durable budget.
func (s *Store) ReserveVisionUnit(ctx context.Context, feature string, monthlyCap int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var used int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(unit_count),0) FROM cost_ledger WHERE service=? AND date >= date('now','start of month')`, feature).Scan(&used)
	if err != nil {
		return false, err
	}
	if used >= monthlyCap {
		return false, nil
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO cost_ledger(date,service,cost,unit_count,notes) VALUES(date('now'),?,0,1,'free-tier vision unit')`, feature); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *Store) RecordCost(ctx context.Context, service string, cost float64, units int, notes string) error {
	if strings.TrimSpace(service) == "" || cost < 0 || units < 0 {
		return errors.New("cost service is required and cost/units must be non-negative")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO cost_ledger(date,service,cost,unit_count,notes) VALUES(date('now'),?,?,?,?)`, service, cost, units, notes)
	return err
}

func (s *Store) CostOverview(ctx context.Context, qualifiedMax float64) (domain.CostOverview, error) {
	overview := domain.CostOverview{MonthlyByService: map[string]float64{}}
	rows, err := s.db.QueryContext(ctx, `SELECT service, COALESCE(SUM(cost),0) FROM cost_ledger WHERE date >= date('now','start of month') GROUP BY service`)
	if err != nil {
		return overview, err
	}
	defer rows.Close()
	for rows.Next() {
		var service string
		var cost float64
		if err := rows.Scan(&service, &cost); err != nil {
			return overview, err
		}
		overview.MonthlyByService[service] = cost
	}
	if err := rows.Err(); err != nil {
		return overview, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(cost),0) FROM cost_ledger WHERE date >= date('now','-30 days')`).Scan(&overview.RunningTotal); err != nil {
		return overview, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN fraud_score IS NOT NULL AND fraud_score <= ? THEN 1 ELSE 0 END),0) FROM listings`, qualifiedMax).Scan(&overview.Processed, &overview.Qualified); err != nil {
		return overview, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT listing_id) FROM notification_deliveries WHERE status='sent'`).Scan(&overview.Notified); err != nil {
		return overview, err
	}
	return overview, nil
}

func (s *Store) SetState(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO application_state(key,value,updated_at) VALUES(?,?,CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=CURRENT_TIMESTAMP`, key, value)
	return err
}
func (s *Store) GetState(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM application_state WHERE key=?`, key).Scan(&v)
	return v, err
}

func (s *Store) RecordModelEvaluation(ctx context.Context, e domain.ModelEvaluation) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO fraud_eval_metrics(eval_date,precision,recall,f1,false_positive_rate,false_negative_rate,total_test_samples,model_version) VALUES(?,?,?,?,?,?,?,?)`, e.EvalDate.Format("2006-01-02"), e.Precision, e.Recall, e.F1, e.FalsePositiveRate, e.FalseNegativeRate, e.TotalSamples, e.ModelVersion)
	return err
}

type delivery struct {
	ID             int64
	ListingID      int64
	IdempotencyKey string
}

func (s *Store) BeginUrgentDelivery(ctx context.Context, listingID int64, key string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO notification_deliveries(listing_id,channel,notification_type,idempotency_key,status) VALUES(?, 'telegram', 'urgent_alert', ?, 'pending')`, listingID, key)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
func (s *Store) FinishUrgentDelivery(ctx context.Context, listingID int64, providerID string, sendErr error) error {
	status, message := "sent", ""
	if sendErr != nil {
		status, message = "failed", sendErr.Error()
	}
	_, err := s.db.ExecContext(ctx, `UPDATE notification_deliveries SET status=?,provider_message_id=?,error_message=?,sent_at=CASE WHEN ?='sent' THEN CURRENT_TIMESTAMP ELSE sent_at END WHERE listing_id=? AND channel='telegram' AND notification_type='urgent_alert'`, status, providerID, message, status, listingID)
	return err
}

func (s *Store) QualifiedWithoutDigest(ctx context.Context, maxFraud float64) ([]domain.Listing, error) {
	rows, err := s.db.QueryContext(ctx, listingSelect+` WHERE fraud_score IS NOT NULL AND fraud_score <= ? AND NOT EXISTS (SELECT 1 FROM notification_deliveries d WHERE d.listing_id=listings.id AND d.channel='resend' AND d.notification_type='digest' AND d.status='sent') ORDER BY seen_at ASC`, maxFraud)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.Listing
	for rows.Next() {
		l, err := scanListing(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

func (s *Store) RecordDigestDeliveries(ctx context.Context, listingIDs []int64, key, providerID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range listingIDs {
		itemKey := fmt.Sprintf("%s:%d", key, id)
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO notification_deliveries(listing_id,channel,notification_type,idempotency_key,provider_message_id,status,sent_at) VALUES(?, 'resend', 'digest', ?, ?, 'sent', CURRENT_TIMESTAMP)`, id, itemKey, providerID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

const listingSelect = `SELECT id,source_id,title,price,bedrooms,bathrooms,location,description,contact_email,contact_phone,posted_at,craigslist_url,fraud_score,fraud_flags,fit_score,urgent_eligible,fraud_evaluation_time,images,reverse_image_matches,reverse_image_source,neighborhood_avg_price,price_percentile,status,user_notes,seen_at,updated_at FROM listings`

type rowScanner interface{ Scan(...any) error }

func scanListing(row rowScanner) (domain.Listing, error) {
	var l domain.Listing
	var flags, images, matches string
	var posted, seen, updated sql.NullTime
	var fraud, fit sql.NullFloat64
	var eval sql.NullInt64
	var location, desc, email, phone, url, source sql.NullString
	var beds, baths sql.NullFloat64
	var urgent int
	var reverseSource sql.NullString
	var neighborhood sql.NullInt64
	var percentile sql.NullFloat64
	err := row.Scan(&l.ID, &source, &l.Title, &l.Price, &beds, &baths, &location, &desc, &email, &phone, &posted, &url, &fraud, &flags, &fit, &urgent, &eval, &images, &matches, &reverseSource, &neighborhood, &percentile, &l.Status, &l.UserNotes, &seen, &updated)
	if err != nil {
		return l, err
	}
	l.SourceID = source.String
	l.Location = location.String
	l.Description = desc.String
	l.ContactEmail = email.String
	l.ContactPhone = phone.String
	l.URL = url.String
	l.UrgentEligible = urgent != 0
	l.ReverseImageSource = reverseSource.String
	if beds.Valid {
		v := beds.Float64
		l.Bedrooms = &v
	}
	if baths.Valid {
		v := baths.Float64
		l.Bathrooms = &v
	}
	if posted.Valid {
		v := posted.Time
		l.PostedAt = &v
	}
	if fraud.Valid {
		v := fraud.Float64
		l.FraudScore = &v
	}
	if fit.Valid {
		v := fit.Float64
		l.FitScore = &v
	}
	if eval.Valid {
		v := eval.Int64
		l.EvaluationMS = &v
	}
	if neighborhood.Valid {
		v := int(neighborhood.Int64)
		l.NeighborhoodPrice = &v
	}
	if percentile.Valid {
		v := percentile.Float64
		l.PricePercentile = &v
	}
	if seen.Valid {
		l.SeenAt = seen.Time
	}
	if updated.Valid {
		l.UpdatedAt = updated.Time
	}
	_ = json.Unmarshal([]byte(flags), &l.FraudFlags)
	_ = json.Unmarshal([]byte(images), &l.Images)
	_ = json.Unmarshal([]byte(matches), &l.ReverseMatches)
	return l, nil
}
func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}
func fallback(v, defaultValue string) string {
	if strings.TrimSpace(v) == "" {
		return defaultValue
	}
	return v
}
