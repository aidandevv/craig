// Package config loads application configuration from the environment and an
// optional .env file. Secrets are never logged or persisted.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr             string
	MetricsAddr            string
	DatabaseURL            string
	FlipifyBaseURL         string
	FlipifyAPIKey          string
	FlipifyPollInterval    time.Duration
	FlipifyQuery           string
	FlipifyLatitude        float64
	FlipifyLongitude       float64
	VisionCredentials      string
	VisionMonthlyCap       int
	InferenceAddress       string
	QualifiedMaxFraudScore float64
	UrgentMaxFraudScore    float64
	UrgentMinFitScore      float64
	PreferredMinPrice      int
	PreferredMaxPrice      int
	PreferredMinBedrooms   float64
	PreferredLocations     []string
	ResendAPIKey           string
	ResendAPIURL           string
	NotificationEmailFrom  string
	NotificationEmailTo    string
	ResendDigestInterval   time.Duration
	TelegramBotToken       string
	TelegramChatID         string
	HTTPTimeout            time.Duration
}

func Load() (Config, error) {
	loadDotEnv(".env")
	c := Config{
		ListenAddr:             env("LISTEN_ADDR", ":8080"),
		MetricsAddr:            env("METRICS_ADDR", ":9090"),
		DatabaseURL:            env("DATABASE_URL", "sqlite:///data/apartments.db"),
		FlipifyBaseURL:         strings.TrimRight(env("FLIPIFY_BASE_URL", "https://api.flipify.com/v1"), "/"),
		FlipifyAPIKey:          os.Getenv("FLIPIFY_API_KEY"),
		FlipifyQuery:           env("FLIPIFY_QUERY", "apartments / housing for rent"),
		VisionCredentials:      os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		VisionMonthlyCap:       envInt("VISION_MONTHLY_IMAGE_CAP", 999),
		InferenceAddress:       env("INFERENCE_ADDRESS", "inference-sidecar:50051"),
		QualifiedMaxFraudScore: envFloat("QUALIFIED_MAX_FRAUD_SCORE", 0.50),
		UrgentMaxFraudScore:    envFloat("URGENT_MAX_FRAUD_SCORE", 0.20),
		UrgentMinFitScore:      envFloat("URGENT_MIN_FIT_SCORE", 0.80),
		PreferredMinPrice:      envInt("PREFERRED_MIN_PRICE", 0),
		PreferredMaxPrice:      envInt("PREFERRED_MAX_PRICE", 0),
		PreferredMinBedrooms:   envFloat("PREFERRED_MIN_BEDROOMS", 0),
		PreferredLocations:     split(env("PREFERRED_LOCATIONS", "")),
		ResendAPIKey:           os.Getenv("RESEND_API_KEY"),
		ResendAPIURL:           env("RESEND_API_URL", "https://api.resend.com"),
		NotificationEmailFrom:  os.Getenv("NOTIFICATION_EMAIL_FROM"),
		NotificationEmailTo:    os.Getenv("NOTIFICATION_EMAIL_TO"),
		TelegramBotToken:       os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:         os.Getenv("TELEGRAM_CHAT_ID"),
		HTTPTimeout:            envDuration("HTTP_TIMEOUT", 10*time.Second),
	}
	var err error
	if c.FlipifyPollInterval, err = envDurationChecked("FLIPIFY_POLL_INTERVAL", 5*time.Minute); err != nil {
		return c, fmt.Errorf("FLIPIFY_POLL_INTERVAL: %w", err)
	}
	if c.ResendDigestInterval, err = envDurationChecked("RESEND_DIGEST_INTERVAL", 12*time.Hour); err != nil {
		return c, fmt.Errorf("RESEND_DIGEST_INTERVAL: %w", err)
	}
	if c.FlipifyLatitude, err = envFloatChecked("FLIPIFY_LATITUDE", 37.7749); err != nil {
		return c, fmt.Errorf("FLIPIFY_LATITUDE: %w", err)
	}
	if c.FlipifyLongitude, err = envFloatChecked("FLIPIFY_LONGITUDE", -122.4194); err != nil {
		return c, fmt.Errorf("FLIPIFY_LONGITUDE: %w", err)
	}
	if c.VisionMonthlyCap < 1 {
		return c, fmt.Errorf("VISION_MONTHLY_IMAGE_CAP must be at least 1")
	}
	if c.QualifiedMaxFraudScore < 0 || c.QualifiedMaxFraudScore > 1 || c.UrgentMaxFraudScore < 0 || c.UrgentMaxFraudScore > 1 || c.UrgentMinFitScore < 0 || c.UrgentMinFitScore > 1 {
		return c, fmt.Errorf("score thresholds must be in [0,1]")
	}
	if c.UrgentMaxFraudScore > c.QualifiedMaxFraudScore {
		return c, fmt.Errorf("URGENT_MAX_FRAUD_SCORE must not exceed QUALIFIED_MAX_FRAUD_SCORE")
	}
	return c, nil
}

func (c Config) FlipifyEnabled() bool { return c.FlipifyAPIKey != "" }
func (c Config) ResendEnabled() bool {
	return c.ResendAPIKey != "" && c.NotificationEmailFrom != "" && c.NotificationEmailTo != ""
}
func (c Config) TelegramEnabled() bool { return c.TelegramBotToken != "" && c.TelegramChatID != "" }

func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func envInt(k string, d int) int {
	v, e := strconv.Atoi(env(k, strconv.Itoa(d)))
	if e != nil {
		return d
	}
	return v
}
func envFloat(k string, d float64) float64 {
	v, e := strconv.ParseFloat(env(k, fmt.Sprint(d)), 64)
	if e != nil {
		return d
	}
	return v
}
func envFloatChecked(k string, d float64) (float64, error) {
	return strconv.ParseFloat(env(k, fmt.Sprint(d)), 64)
}
func envDuration(k string, d time.Duration) time.Duration {
	v, e := time.ParseDuration(env(k, d.String()))
	if e != nil {
		return d
	}
	return v
}
func envDurationChecked(k string, d time.Duration) (time.Duration, error) {
	return time.ParseDuration(env(k, d.String()))
}
func split(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(strings.ToLower(p)); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pair := strings.SplitN(line, "=", 2)
		if len(pair) != 2 || os.Getenv(pair[0]) != "" {
			continue
		}
		_ = os.Setenv(strings.TrimSpace(pair[0]), strings.Trim(strings.TrimSpace(pair[1]), "\"'"))
	}
}
