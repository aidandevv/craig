package database

const schema = `
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS listings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  source_id TEXT UNIQUE NOT NULL,
  title TEXT NOT NULL,
  price INTEGER NOT NULL DEFAULT 0,
  bedrooms REAL,
  bathrooms REAL,
  location TEXT,
  description TEXT,
  contact_email TEXT,
  contact_phone TEXT,
  posted_at DATETIME,
  craigslist_url TEXT,
  fraud_score REAL,
  fraud_flags TEXT NOT NULL DEFAULT '[]',
  fit_score REAL,
  urgent_eligible BOOLEAN NOT NULL DEFAULT FALSE,
  fraud_evaluation_time INTEGER,
  image_count INTEGER NOT NULL DEFAULT 0,
  images TEXT NOT NULL DEFAULT '[]',
  reverse_image_matches TEXT NOT NULL DEFAULT '[]',
  reverse_image_source TEXT,
  neighborhood_avg_price INTEGER,
  price_percentile REAL,
  comps_fetched_at DATETIME,
  status TEXT NOT NULL DEFAULT 'new',
  user_notes TEXT NOT NULL DEFAULT '',
  contacted_at DATETIME,
  toured_at DATETIME,
  applied_at DATETIME,
  seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS fraud_eval_metrics (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  eval_date DATE NOT NULL,
  precision REAL NOT NULL,
  recall REAL NOT NULL,
  f1 REAL NOT NULL,
  false_positive_rate REAL NOT NULL,
  false_negative_rate REAL NOT NULL,
  total_test_samples INTEGER NOT NULL,
  model_version TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS cost_ledger (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  date DATE NOT NULL,
  service TEXT NOT NULL,
  cost REAL NOT NULL DEFAULT 0,
  unit_count INTEGER NOT NULL DEFAULT 0,
  notes TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS notification_deliveries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  listing_id INTEGER NOT NULL REFERENCES listings(id),
  channel TEXT NOT NULL,
  notification_type TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  provider_message_id TEXT,
  status TEXT NOT NULL,
  error_message TEXT,
  sent_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(listing_id, channel, notification_type),
  UNIQUE(idempotency_key)
);
CREATE TABLE IF NOT EXISTS image_signal_cache (
  image_hash TEXT NOT NULL,
  signal TEXT NOT NULL,
  result TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(image_hash, signal)
);
CREATE TABLE IF NOT EXISTS application_state (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_listings_fraud_score ON listings(fraud_score);
CREATE INDEX IF NOT EXISTS idx_listings_status ON listings(status);
CREATE INDEX IF NOT EXISTS idx_listings_seen_at ON listings(seen_at);
CREATE INDEX IF NOT EXISTS idx_cost_ledger_date_service ON cost_ledger(date, service);
`
