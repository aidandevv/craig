// Package notifications delivers the two intentional channels: Telegram for
// exceptional new listings and Resend for the complete qualified-listing digest.
package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/config"
	"github.com/aidandevaney/apartment-hunter/internal/database"
	"github.com/aidandevaney/apartment-hunter/internal/domain"
	"github.com/aidandevaney/apartment-hunter/internal/metrics"
)

type Service struct {
	store   *database.Store
	config  config.Config
	metrics *metrics.Metrics
	logger  *slog.Logger
	client  *http.Client
}

func New(store *database.Store, c config.Config, metric *metrics.Metrics, logger *slog.Logger) *Service {
	return &Service{store: store, config: c, metrics: metric, logger: logger, client: &http.Client{Timeout: c.HTTPTimeout}}
}

func (s *Service) HandleNewQualified(ctx context.Context, listing domain.Listing, result domain.FraudResult) {
	if !result.Urgent || !s.config.TelegramEnabled() {
		return
	}
	key := fmt.Sprintf("telegram-urgent:%d", listing.ID)
	send, err := s.store.BeginUrgentDelivery(ctx, listing.ID, key)
	if err != nil {
		s.logger.Error("reserve Telegram delivery", "listing_id", listing.ID, "error", err)
		return
	}
	if !send {
		return
	}
	providerID, err := s.sendTelegram(ctx, listing, result)
	if err != nil {
		s.logger.Error("send Telegram alert", "listing_id", listing.ID, "error", err)
		s.metrics.Notifications.WithLabelValues("telegram", "failed").Inc()
	} else {
		s.metrics.Notifications.WithLabelValues("telegram", "sent").Inc()
	}
	if updateErr := s.store.FinishUrgentDelivery(ctx, listing.ID, providerID, err); updateErr != nil {
		s.logger.Error("record Telegram result", "listing_id", listing.ID, "error", updateErr)
	}
}

func (s *Service) RunDigestLoop(ctx context.Context) {
	if !s.config.ResendEnabled() {
		s.logger.Warn("Resend digest disabled: email credentials are incomplete")
		return
	}
	ticker := time.NewTicker(s.config.ResendDigestInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.SendDigest(ctx)
		}
	}
}

func (s *Service) SendDigest(ctx context.Context) {
	if !s.config.ResendEnabled() {
		return
	}
	listings, err := s.store.QualifiedWithoutDigest(ctx, s.config.QualifiedMaxFraudScore)
	if err != nil {
		s.logger.Error("load digest listings", "error", err)
		return
	}
	if len(listings) == 0 {
		return
	}
	window := time.Now().UTC().Unix() / int64(s.config.ResendDigestInterval.Seconds())
	key := fmt.Sprintf("resend-digest:%d", window)
	providerID, err := s.sendResend(ctx, listings, key)
	if err != nil {
		s.logger.Error("send Resend digest", "error", err)
		s.metrics.Notifications.WithLabelValues("resend", "failed").Inc()
		return
	}
	ids := make([]int64, 0, len(listings))
	for _, listing := range listings {
		ids = append(ids, listing.ID)
	}
	if err := s.store.RecordDigestDeliveries(ctx, ids, key, providerID); err != nil {
		s.logger.Error("record digest deliveries", "error", err)
		return
	}
	s.metrics.Notifications.WithLabelValues("resend", "sent").Add(float64(len(listings)))
}

func (s *Service) sendTelegram(ctx context.Context, l domain.Listing, r domain.FraudResult) (string, error) {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.config.TelegramBotToken)
	reason := "low fraud risk and strong preference match"
	text := fmt.Sprintf("🏠 Urgent Apartment Hunter match\n%s\n$%d · %s\nFraud risk: %.0f%% · Fit: %.0f%%\nReason: %s\n%s", l.Title, l.Price, fallback(l.Location, "location unavailable"), r.Score*100, r.FitScore*100, reason, l.URL)
	body, _ := json.Marshal(map[string]string{"chat_id": s.config.TelegramChatID, "text": text, "disable_web_page_preview": "true"})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("Telegram returned HTTP %d: %s", response.StatusCode, truncate(string(data), 300))
	}
	var decoded struct {
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	return fmt.Sprint(decoded.Result.MessageID), nil
}

func (s *Service) sendResend(ctx context.Context, listings []domain.Listing, idempotencyKey string) (string, error) {
	items := make([]string, 0, len(listings))
	plain := []string{"Qualified Apartment Hunter listings:"}
	for _, l := range listings {
		items = append(items, fmt.Sprintf("<li><a href=%q>%s</a> — $%d · %s · fraud risk %.0f%%</li>", html.EscapeString(l.URL), html.EscapeString(l.Title), l.Price, html.EscapeString(fallback(l.Location, "location unavailable")), value(l.FraudScore)*100))
		plain = append(plain, fmt.Sprintf("- %s — $%d · %s · fraud risk %.0f%%\n  %s", l.Title, l.Price, fallback(l.Location, "location unavailable"), value(l.FraudScore)*100, l.URL))
	}
	payload := map[string]string{"from": s.config.NotificationEmailFrom, "to": s.config.NotificationEmailTo, "subject": fmt.Sprintf("Apartment Hunter: %d qualified listing(s)", len(listings)), "html": "<h2>Qualified listings</h2><ul>" + strings.Join(items, "") + "</ul>", "text": strings.Join(plain, "\n")}
	raw, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.config.ResendAPIURL, "/")+"/emails", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+s.config.ResendAPIKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("Resend returned HTTP %d: %s", response.StatusCode, truncate(string(data), 300))
	}
	var decoded struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	return decoded.ID, nil
}
func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
func value(pointer *float64) float64 {
	if pointer == nil {
		return 0
	}
	return *pointer
}
func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}
