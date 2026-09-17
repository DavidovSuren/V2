const express = require('express');
const db = require('../db');
const { identify, requireUser } = require('../lib/telegramAuth');
const { progressFromCompleted, categoryProgressPct, TOTAL_TASKS } = require('../lib/leveling');
const { categoryTotals } = require('../lib/taskBank');
const { ACHIEVEMENT_META } = require('../lib/achievements');

const router = express.Router();

router.get('/progress', identify, requireUser, (req, res) => {
  const user = req.user;
  const { progressPct, level } = progressFromCompleted(user.completed_count);

  const doneRows = db.prepare(`
    SELECT category, COUNT(*) as done FROM user_schedule
    WHERE user_id = ? AND status = 'done' GROUP BY category
  `).all(user.id);
  const doneByCategory = {};
  doneRows.forEach(r => { doneByCategory[r.category] = r.done; });

  const totals = user.gender ? categoryTotals(user.gender) : {};
  const categories = Object.keys(totals).map(cat => ({
    name: cat,
    pct: categoryProgressPct(doneByCategory[cat] || 0, totals[cat]),
    done: doneByCategory[cat] || 0,
    total: totals[cat]
  }));

  res.json({
    level, progressPct,
    xp: user.xp,
    completedCount: user.completed_count,
    totalTasks: TOTAL_TASKS,
    streakCurrent: user.streak_current,
    streakBest: user.streak_best,
    categories
  });
});

router.get('/achievements', identify, requireUser, (req, res) => {
  const rows = db.prepare('SELECT code, unlocked_at FROM achievements WHERE user_id = ?').all(req.user.id);
  const unlockedCodes = new Set(rows.map(r => r.code));
  const all = Object.entries(ACHIEVEMENT_META).map(([code, meta]) => ({
    code, ...meta, unlocked: unlockedCodes.has(code)
  }));
  res.json({ achievements: all });
});

module.exports = router;
