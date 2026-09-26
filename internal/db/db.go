// Package db открывает пул подключений к PostgreSQL и применяет схему.
package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func Open(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		log.Println("[db] DATABASE_URL не задан — подключение к PostgreSQL не удастся.")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	return db, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  tg_id TEXT UNIQUE NOT NULL,
  username TEXT,
  name TEXT,
  gender TEXT CHECK(gender IN ('male','female')),
  age_group TEXT,
  photos_json TEXT DEFAULT '[]',
  created_at TEXT NOT NULL,

  level INTEGER NOT NULL DEFAULT 0,
  xp INTEGER NOT NULL DEFAULT 0,
  completed_count INTEGER NOT NULL DEFAULT 0,

  streak_current INTEGER NOT NULL DEFAULT 0,
  streak_best INTEGER NOT NULL DEFAULT 0,

  day_index INTEGER NOT NULL DEFAULT 0,
  last_action_date TEXT,

  subscription_tier TEXT NOT NULL DEFAULT 'free',
  subscription_expires_at TEXT,

  referral_code TEXT UNIQUE,
  premium_agent_code TEXT UNIQUE,
  referred_by_user_id INTEGER REFERENCES users(id),
  referred_by_code_type TEXT
);

CREATE TABLE IF NOT EXISTS quiz_answers (
  user_id INTEGER NOT NULL REFERENCES users(id),
  question_id INTEGER NOT NULL,
  answer_json TEXT NOT NULL,
  PRIMARY KEY (user_id, question_id)
);

CREATE TABLE IF NOT EXISTS category_weights (
  user_id INTEGER NOT NULL REFERENCES users(id),
  category TEXT NOT NULL,
  weight REAL NOT NULL,
  PRIMARY KEY (user_id, category)
);

CREATE TABLE IF NOT EXISTS user_schedule (
  user_id INTEGER NOT NULL REFERENCES users(id),
  day_index INTEGER NOT NULL,
  task_id TEXT NOT NULL,
  category TEXT NOT NULL,
  task_text TEXT NOT NULL,
  task_why TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  PRIMARY KEY (user_id, day_index)
);

CREATE TABLE IF NOT EXISTS diary_entries (
  user_id INTEGER NOT NULL REFERENCES users(id),
  entry_date TEXT NOT NULL,
  emoji TEXT NOT NULL,
  note TEXT,
  xp_awarded INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, entry_date)
);

CREATE TABLE IF NOT EXISTS action_log (
  user_id INTEGER NOT NULL REFERENCES users(id),
  action_date TEXT NOT NULL,
  action TEXT NOT NULL CHECK(action IN ('done','skip')),
  category TEXT,
  PRIMARY KEY (user_id, action_date)
);

CREATE TABLE IF NOT EXISTS achievements (
  user_id INTEGER NOT NULL REFERENCES users(id),
  code TEXT NOT NULL,
  unlocked_at TEXT NOT NULL,
  PRIMARY KEY (user_id, code)
);

CREATE TABLE IF NOT EXISTS referrals (
  id SERIAL PRIMARY KEY,
  referrer_id INTEGER NOT NULL REFERENCES users(id),
  referred_id INTEGER NOT NULL UNIQUE REFERENCES users(id),
  code_type TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS subscription_payments (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id),
  tier TEXT NOT NULL,
  base_price INTEGER NOT NULL,
  discount_pct INTEGER NOT NULL DEFAULT 0,
  price_paid INTEGER NOT NULL,
  referrer_commission_user_id INTEGER REFERENCES users(id),
  commission_amount INTEGER NOT NULL DEFAULT 0,
  paid_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS wallet_transactions (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id),
  amount INTEGER NOT NULL,
  source_user_id INTEGER REFERENCES users(id),
  note TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS friendships (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id),
  friend_id INTEGER NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL,
  UNIQUE(user_id, friend_id)
);

CREATE INDEX IF NOT EXISTS idx_action_log_user ON action_log(user_id);
-- Промокоды (например "100LVL") — один раз на пользователя на код,
-- чтобы нельзя было повторно применить и, например, бесконечно продлевать
-- подарочный Premium за 100 уровень.
CREATE TABLE IF NOT EXISTS promo_redemptions (
  user_id INTEGER NOT NULL REFERENCES users(id),
  code TEXT NOT NULL,
  redeemed_at TEXT NOT NULL,
  PRIMARY KEY (user_id, code)
);

-- Редактируемый из админки контент. При старте сюда досеиваются
-- вопросы из кода и задания из data/*.json (только отсутствующие id),
-- дальше источник правды — БД. Правки заданий влияют только на новые планы:
-- user_schedule хранит копию текста.
CREATE TABLE IF NOT EXISTS questions (
  id INTEGER PRIMARY KEY,
  category TEXT NOT NULL DEFAULT '',
  text TEXT NOT NULL,
  type TEXT NOT NULL,
  diagnostic BOOLEAN NOT NULL,
  options_json TEXT NOT NULL DEFAULT '[]',
  updated_at TEXT
);

CREATE TABLE IF NOT EXISTS tasks (
  bank TEXT NOT NULL CHECK(bank IN ('male','female')),
  id TEXT NOT NULL,
  position INTEGER NOT NULL,
  category TEXT NOT NULL,
  gender TEXT NOT NULL,
  text TEXT NOT NULL,
  why TEXT NOT NULL,
  updated_at TEXT,
  PRIMARY KEY (bank, id)
);

-- Пароль кабинета агента (/partner), bcrypt.
ALTER TABLE users ADD COLUMN IF NOT EXISTS agent_password_hash TEXT;

-- Партнёрская программа (этап 1): процент, по которому начислена комиссия,
-- и идентификатор платежа Telegram — повторный successful_payment с тем же
-- charge_id не создаёт вторую оплату.
ALTER TABLE subscription_payments ADD COLUMN IF NOT EXISTS commission_pct INTEGER NOT NULL DEFAULT 0;
ALTER TABLE subscription_payments ADD COLUMN IF NOT EXISTS charge_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_charge ON subscription_payments(charge_id);
CREATE INDEX IF NOT EXISTS idx_users_referred_by ON users(referred_by_user_id);

-- Вывод из кошелька (этап 4): раз в месяц, 15-го, с удержанием НДФЛ.
-- Номер карты хранится только зашифрованным (AES-256-GCM, CARD_ENC_KEY).
CREATE TABLE IF NOT EXISTS withdrawals (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id),
  month_key TEXT NOT NULL,
  gross INTEGER NOT NULL,
  tax INTEGER NOT NULL,
  net INTEGER NOT NULL,
  tax_pct INTEGER NOT NULL,
  card_last4 TEXT NOT NULL,
  card_enc TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','paid','rejected')),
  created_at TEXT NOT NULL,
  processed_at TEXT,
  reject_reason TEXT
);
-- Один вывод в месяц; отклонённая заявка не мешает подать новую.
CREATE UNIQUE INDEX IF NOT EXISTS idx_withdrawals_month ON withdrawals(user_id, month_key) WHERE status <> 'rejected';
ALTER TABLE users ADD COLUMN IF NOT EXISTS payout_card_enc TEXT;
-- Пользовательское соглашение (этап 5): когда и какую редакцию принял.
ALTER TABLE users ADD COLUMN IF NOT EXISTS terms_accepted_at TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS terms_version TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS payout_card_last4 TEXT;
ALTER TABLE wallet_transactions ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'income';
ALTER TABLE wallet_transactions ADD COLUMN IF NOT EXISTS withdrawal_id INTEGER REFERENCES withdrawals(id);

-- Награды «Первый шаг» и «Уровень 10» (этап 7) — тем, кто уже их заслужил.
INSERT INTO achievements (user_id, code, unlocked_at)
  SELECT id, 'first', created_at FROM users WHERE completed_count >= 1
  ON CONFLICT (user_id, code) DO NOTHING;
INSERT INTO achievements (user_id, code, unlocked_at)
  SELECT id, 'lvl10', created_at FROM users WHERE level >= 10
  ON CONFLICT (user_id, code) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_schedule_user ON user_schedule(user_id);
CREATE INDEX IF NOT EXISTS idx_diary_user ON diary_entries(user_id);
CREATE INDEX IF NOT EXISTS idx_referrals_referrer ON referrals(referrer_id);
CREATE INDEX IF NOT EXISTS idx_payments_user ON subscription_payments(user_id);
CREATE INDEX IF NOT EXISTS idx_wallet_user ON wallet_transactions(user_id);
`

func Migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
