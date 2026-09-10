package cache

const schema = `
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS image_signal_cache (
  image_hash TEXT NOT NULL,
  signal     TEXT NOT NULL,
  result     TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(image_hash, signal)
);

CREATE TABLE IF NOT EXISTS vision_ledger (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  date       DATE NOT NULL,
  feature    TEXT NOT NULL,
  unit_count INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_vision_ledger_date_feature ON vision_ledger(date, feature);
CREATE INDEX IF NOT EXISTS idx_image_signal_cache_created ON image_signal_cache(created_at);
`
