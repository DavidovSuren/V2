const cron = require('node-cron');
const db = require('../db');
const { todayMoscow } = require('../lib/dateUtil');
const { sendMessage } = require('../lib/telegram');

// Каждый день в 15:15 по московскому времени — напоминание тем, кто ещё
// не отметил сегодняшнее действие.
function start() {
  cron.schedule('15 15 * * *', async () => {
    const today = todayMoscow();
    const users = await db.all(`
      SELECT id, tg_id, day_index FROM users
      WHERE gender IS NOT NULL AND day_index < 365
        AND (last_action_date IS NULL OR last_action_date != ?)
    `, [today]);

    for (const user of users) {
      const row = await db.get('SELECT task_text FROM user_schedule WHERE user_id = ? AND day_index = ?',
        [user.id, user.day_index]);
      if (!row) continue;
      await sendMessage(user.tg_id, `⏰ Напоминание Version 2.0\n\nСегодняшнее действие:\n${row.task_text}\n\nОткрой приложение и отметь, выполнил(а) ли ты его.`);
    }
  }, { timezone: 'Europe/Moscow' });

  console.log('[cron] dailyReminder запланирован на 15:15 Europe/Moscow');
}

module.exports = { start };
