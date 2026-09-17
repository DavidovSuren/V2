const db = require('../db');
const { todayMoscow } = require('./dateUtil');
const { WEEKLY_REPORT_XP, MONTHLY_REPORT_XP } = require('./leveling');

function dateNDaysAgo(n) {
  const d = new Date(todayMoscow() + 'T00:00:00Z');
  d.setUTCDate(d.getUTCDate() - n);
  return d.toISOString().slice(0, 10);
}

async function topWeightedRemainingCategory(userId) {
  const row = await db.get(`
    SELECT cw.category FROM category_weights cw
    WHERE cw.user_id = ? AND EXISTS (
      SELECT 1 FROM user_schedule us
      WHERE us.user_id = cw.user_id AND us.category = cw.category AND us.status = 'pending'
    )
    ORDER BY cw.weight DESC LIMIT 1
  `, [userId]);
  return row ? row.category : null;
}

async function weeklyReportFor(userId) {
  const from = dateNDaysAgo(6);
  const user = await db.get('SELECT * FROM users WHERE id = ?', [userId]);

  const counts = await db.all(`
    SELECT action, COUNT(*)::int as n FROM action_log
    WHERE user_id = ? AND action_date >= ? GROUP BY action
  `, [userId, from]);
  const done = counts.find(c => c.action === 'done')?.n || 0;
  const skipped = counts.find(c => c.action === 'skip')?.n || 0;

  const topCategoryRow = await db.get(`
    SELECT category, COUNT(*)::int as n FROM action_log
    WHERE user_id = ? AND action_date >= ? AND action = 'done'
    GROUP BY category ORDER BY n DESC LIMIT 1
  `, [userId, from]);

  return {
    from, to: todayMoscow(),
    completed: done,
    skipped,
    activeDays: done + skipped,
    streakCurrent: user.streak_current,
    topCategory: topCategoryRow ? topCategoryRow.category : null,
    focusNextWeek: await topWeightedRemainingCategory(userId),
    xpAwarded: WEEKLY_REPORT_XP
  };
}

async function monthlyReportFor(userId) {
  const from = dateNDaysAgo(29);
  const user = await db.get('SELECT * FROM users WHERE id = ?', [userId]);

  const counts = await db.all(`
    SELECT action, COUNT(*)::int as n FROM action_log
    WHERE user_id = ? AND action_date >= ? GROUP BY action
  `, [userId, from]);
  const done = counts.find(c => c.action === 'done')?.n || 0;
  const skipped = counts.find(c => c.action === 'skip')?.n || 0;

  const categoryBreakdown = await db.all(`
    SELECT category, COUNT(*)::int as n FROM action_log
    WHERE user_id = ? AND action_date >= ? AND action = 'done'
    GROUP BY category ORDER BY n DESC
  `, [userId, from]);

  const moodRows = await db.all(`
    SELECT entry_date, emoji FROM diary_entries
    WHERE user_id = ? AND entry_date >= ? ORDER BY entry_date ASC
  `, [userId, from]);

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
