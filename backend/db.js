const { Pool } = require('pg');

if (!process.env.DATABASE_URL) {
  console.warn('[db] DATABASE_URL не задан — подключение к PostgreSQL не удастся.');
}

const pool = new Pool({ connectionString: process.env.DATABASE_URL });

// Позволяет писать SQL с плейсхолдерами '?' (как раньше с better-sqlite3) —
// здесь они конвертируются в позиционные $1, $2, ... для pg.
function toPg(sql) {
  let i = 0;
  return sql.replace(/\?/g, () => `$${++i}`);
}

async function get(sql, params = []) {
  const res = await pool.query(toPg(sql), params);
  return res.rows[0];
}

async function all(sql, params = []) {
  const res = await pool.query(toPg(sql), params);
  return res.rows;
}

async function run(sql, params = []) {
  const res = await pool.query(toPg(sql), params);
  return { rowCount: res.rowCount, rows: res.rows };
}

// Транзакция на выделенном клиенте пула — get/all/run внутри fn(t) идут
// через тот же клиент, поэтому BEGIN/COMMIT видят все изменения разом.
async function transaction(fn) {
  const client = await pool.connect();
  try {
    await client.query('BEGIN');
    const t = {
      get: async (sql, params = []) => (await client.query(toPg(sql), params)).rows[0],
      all: async (sql, params = []) => (await client.query(toPg(sql), params)).rows,
      run: async (sql, params = []) => {
        const r = await client.query(toPg(sql), params);
        return { rowCount: r.rowCount, rows: r.rows };
      }
    };
    const result = await fn(t);
    await client.query('COMMIT');
    return result;
  } catch (err) {
    await client.query('ROLLBACK');
    throw err;
  } finally {
    client.release();
  }
}

async function migrate() {
  await pool.query(`
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

    -- Задание каждого дня хранится денормализованно (task_text/task_why
    -- скопированы из банка на момент построения плана). "Пропуск" НЕ
    -- двигает day_index — тот же день просто остаётся текущим.
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

    -- Один ряд на календарный день (МСК) — используется для еженедельных и
    -- ежемесячных отчётов (сколько выполнено / пропущено за период).
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
    CREATE INDEX IF NOT EXISTS idx_schedule_user ON user_schedule(user_id);
    CREATE INDEX IF NOT EXISTS idx_diary_user ON diary_entries(user_id);
    CREATE INDEX IF NOT EXISTS idx_referrals_referrer ON referrals(referrer_id);
    CREATE INDEX IF NOT EXISTS idx_payments_user ON subscription_payments(user_id);
    CREATE INDEX IF NOT EXISTS idx_wallet_user ON wallet_transactions(user_id);
  `);
}

module.exports = { pool, get, all, run, transaction, migrate };
