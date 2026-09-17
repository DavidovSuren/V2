const express = require('express');
const db = require('../db');
const { identify, requireUser } = require('../lib/telegramAuth');
const { todayMoscow, isYesterday } = require('../lib/dateUtil');
const { progressFromCompleted, xpForTaskCompletion } = require('../lib/leveling');
const { checkAndUnlock, ACHIEVEMENT_META } = require('../lib/achievements');

const router = express.Router();

const VALID_EMOJI = ['1', '2', '3', '4', '5'];

// Сохранение дневника — это же и есть "выполнил(а) задание": в исходном
// UX кнопка "Выполнил(а)" ведёт в дневник, а реально засчитывает прогресс
// именно сохранение дневника (эмодзи + заметка).
router.post('/diary', identify, requireUser, (req, res) => {
  const { emoji, note } = req.body || {};
  if (!VALID_EMOJI.includes(String(emoji))) {
    return res.status(400).json({ error: 'Нужно выбрать эмоцию' });
  }

  const user = req.user;
  const today = todayMoscow();

  if (user.day_index >= 365) {
    return res.status(409).json({ error: 'Все 365 заданий уже выполнены — это финал пути' });
  }
  if (user.last_action_date === today) {
    return res.status(409).json({ error: 'Сегодня действие уже отмечено — заходи завтра' });
  }

  const scheduleRow = db.prepare('SELECT * FROM user_schedule WHERE user_id = ? AND day_index = ?')
    .get(user.id, user.day_index);
  if (!scheduleRow) return res.status(409).json({ error: 'Анкета ещё не пройдена' });

  const newStreak = isYesterday(user.last_action_date, today) ? user.streak_current + 1 : 1;
  const streakBest = Math.max(user.streak_best, newStreak);
  const completedCount = user.completed_count + 1;
  const xpGain = xpForTaskCompletion(newStreak);
  const { level, progressPct } = progressFromCompleted(completedCount);
  const wasBelow100 = user.level < 100;

  const tx = db.transaction(() => {
    db.prepare('UPDATE user_schedule SET status = ? WHERE user_id = ? AND day_index = ?')
      .run('done', user.id, user.day_index);

    db.prepare(`
      INSERT INTO diary_entries (user_id, entry_date, emoji, note, xp_awarded)
      VALUES (?, ?, ?, ?, ?)
    `).run(user.id, today, String(emoji), note || null, xpGain);

    db.prepare(`
      INSERT INTO action_log (user_id, action_date, action, category) VALUES (?, ?, 'done', ?)
      ON CONFLICT(user_id, action_date) DO NOTHING
    `).run(user.id, today, scheduleRow.category);

    db.prepare(`
      UPDATE users SET
        completed_count = ?, xp = xp + ?, level = ?,
        streak_current = ?, streak_best = ?,
        day_index = day_index + 1, last_action_date = ?
      WHERE id = ?
    `).run(completedCount, xpGain, level, newStreak, streakBest, today, user.id);

    if (level >= 100 && wasBelow100) {
      const expires = new Date();
      expires.setMonth(expires.getMonth() + 6);
      db.prepare(`
        UPDATE users SET subscription_tier = 'premium888', subscription_expires_at = ?
        WHERE id = ?
      `).run(expires.toISOString(), user.id);
    }
  });
  tx();

  const updatedUser = db.prepare('SELECT * FROM users WHERE id = ?').get(user.id);
  const unlockedCodes = checkAndUnlock(updatedUser);
  const unlocked = unlockedCodes.map(code => ({ code, ...ACHIEVEMENT_META[code] }));
  const reachedLevel100 = level >= 100 && wasBelow100;

  res.json({
    ok: true,
    xpGain,
    level,
    progressPct,
    streakCurrent: newStreak,
    completedCount,
    unlocked,
    reachedLevel100,
    premiumGrantedMonths: reachedLevel100 ? 6 : 0
  });
});

router.get('/diary', identify, requireUser, (req, res) => {
  const limit = Math.min(180, Number(req.query.limit) || 60);
  const rows = db.prepare(`
    SELECT entry_date, emoji, note FROM diary_entries
    WHERE user_id = ? ORDER BY entry_date DESC LIMIT ?
  `).all(req.user.id, limit);
  res.json({ entries: rows });
});

module.exports = router;
