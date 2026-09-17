const express = require('express');
const db = require('../db');
const { identify, requireUser } = require('../lib/telegramAuth');
const { todayMoscow } = require('../lib/dateUtil');

const router = express.Router();

// Задание "на сегодня" — это просто текущая строка плана (day_index).
// Оно НЕ меняется само по себе: либо пользователь его выполняет (тогда
// day_index сдвигается в POST /api/diary), либо пропускает (тогда
// day_index остаётся тем же — то же задание будет "сегодняшним" и завтра).
router.get('/today', identify, requireUser, (req, res) => {
  const user = req.user;

  if (user.day_index >= 365) {
    const lastRow = db.prepare('SELECT * FROM user_schedule WHERE user_id = ? AND day_index = 364').get(user.id);
    return res.json({
      dayIndex: 364,
      category: lastRow?.category || null,
      text: lastRow?.task_text || null,
      why: lastRow?.task_why || null,
      status: 'done',
      finished: true,
      canActToday: false
    });
  }

  const row = db.prepare('SELECT * FROM user_schedule WHERE user_id = ? AND day_index = ?')
    .get(user.id, user.day_index);

  if (!row) {
    return res.status(409).json({ error: 'Анкета ещё не пройдена', quizDone: false });
  }

  res.json({
    dayIndex: user.day_index,
    category: row.category,
    text: row.task_text,
    why: row.task_why,
    status: row.status,
    finished: false,
    canActToday: user.last_action_date !== todayMoscow()
  });
});

router.post('/today/skip', identify, requireUser, (req, res) => {
  const user = req.user;
  const today = todayMoscow();

  if (user.day_index >= 365) {
    return res.status(409).json({ error: 'Все 365 заданий уже выполнены' });
  }
  if (user.last_action_date === today) {
    return res.status(409).json({ error: 'Сегодня действие уже отмечено — заходи завтра' });
  }

  const row = db.prepare('SELECT category FROM user_schedule WHERE user_id = ? AND day_index = ?')
    .get(user.id, user.day_index);

  const tx = db.transaction(() => {
    db.prepare('UPDATE users SET last_action_date = ?, streak_current = 0 WHERE id = ?')
      .run(today, user.id);
    db.prepare(`
      INSERT INTO action_log (user_id, action_date, action, category) VALUES (?, ?, 'skip', ?)
      ON CONFLICT(user_id, action_date) DO NOTHING
    `).run(user.id, today, row ? row.category : null);
  });
  tx();

  res.json({ ok: true, streakCurrent: 0 });
});

module.exports = router;
