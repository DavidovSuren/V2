const db = require('../db');
const { todayMoscow } = require('./dateUtil');
const { WEEKLY_REPORT_XP, MONTHLY_REPORT_XP } = require('./leveling');

function dateNDaysAgo(n) {
  const d = new Date(todayMoscow() + 'T00:00:00Z');
  d.setUTCDate(d.getUTCDate() - n);
  return d.toISOString().slice(0, 10);
}

function topWeightedRemainingCategory(userId) {
  const row = db.prepare(`
    SELECT cw.category FROM category_weights cw
    WHERE cw.user_id = ? AND EXISTS (
      SELECT 1 FROM user_schedule us
      WHERE us.user_id = cw.user_id AND us.category = cw.category AND us.status = 'pending'
    )
    ORDER BY cw.weight DESC LIMIT 1
  `).get(userId);
  return row ? row.category : null;
}

function weeklyReportFor(userId) {
  const from = dateNDaysAgo(6);
  const user = db.prepare('SELECT * FROM users WHERE id = ?').get(userId);

  const counts = db.prepare(`
    SELECT action, COUNT(*) as n FROM action_log
    WHERE user_id = ? AND action_date >= ? GROUP BY action
  `).all(userId, from);
  const done = counts.find(c => c.action === 'done')?.n || 0;
  const skipped = counts.find(c => c.action === 'skip')?.n || 0;

  const topCategoryRow = db.prepare(`
    SELECT category, COUNT(*) as n FROM action_log
    WHERE user_id = ? AND action_date >= ? AND action = 'done'
    GROUP BY category ORDER BY n DESC LIMIT 1
  `).get(userId, from);

  return {
    from, to: todayMoscow(),
    completed: done,
    skipped,
    activeDays: done + skipped,
    streakCurrent: user.streak_current,
    topCategory: topCategoryRow ? topCategoryRow.category : null,
    focusNextWeek: topWeightedRemainingCategory(userId),
    xpAwarded: WEEKLY_REPORT_XP
  };
}

function monthlyReportFor(userId) {
  const from = dateNDaysAgo(29);
  const user = db.prepare('SELECT * FROM users WHERE id = ?').get(userId);

  const counts = db.prepare(`
    SELECT action, COUNT(*) as n FROM action_log
    WHERE user_id = ? AND action_date >= ? GROUP BY action
  `).all(userId, from);
  const done = counts.find(c => c.action === 'done')?.n || 0;
  const skipped = counts.find(c => c.action === 'skip')?.n || 0;

  const categoryBreakdown = db.prepare(`
    SELECT category, COUNT(*) as n FROM action_log
    WHERE user_id = ? AND action_date >= ? AND action = 'done'
    GROUP BY category ORDER BY n DESC
  `).all(userId, from);

  const moodRows = db.prepare(`
    SELECT entry_date, emoji FROM diary_entries
    WHERE user_id = ? AND entry_date >= ? ORDER BY entry_date ASC
  `).all(userId, from);

  // Динамика настроения по неделям месяца — среднее значение эмодзи (1..5).
  const weeks = [[], [], [], [], []];
  const fromDate = new Date(from + 'T00:00:00Z');
  moodRows.forEach(r => {
    const d = new Date(r.entry_date + 'T00:00:00Z');
    const weekIdx = Math.min(4, Math.floor((d - fromDate) / (7 * 86400000)));
    weeks[weekIdx].push(Number(r.emoji));
  });
  const moodTrend = weeks
    .map((vals, i) => vals.length ? { week: i + 1, avgMood: Math.round((vals.reduce((a, b) => a + b, 0) / vals.length) * 10) / 10 } : null)
    .filter(Boolean);

  return {
    from, to: todayMoscow(),
    completed: done,
    skipped,
    streakCurrent: user.streak_current,
    streakBest: user.streak_best,
    categoryBreakdown,
    moodTrend,
    xpAwarded: MONTHLY_REPORT_XP
  };
}

module.exports = { weeklyReportFor, monthlyReportFor };
