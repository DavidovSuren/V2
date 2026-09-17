const cron = require('node-cron');
const db = require('../db');
const { weeklyReportFor } = require('../lib/reports');
const { WEEKLY_REPORT_XP } = require('../lib/leveling');
const { sendMessage } = require('../lib/telegram');

// Каждый понедельник в 10:00 по московскому времени — отчёт за неделю + XP.
function start() {
  cron.schedule('0 10 * * 1', async () => {
    const users = db.prepare('SELECT id, tg_id FROM users WHERE gender IS NOT NULL').all();

    for (const user of users) {
      const report = weeklyReportFor(user.id);
      db.prepare('UPDATE users SET xp = xp + ? WHERE id = ?').run(WEEKLY_REPORT_XP, user.id);

      const text = `📊 Недельный отчёт\n\n` +
        `Выполнено заданий: ${report.completed} / 7\n` +
        `Пропущено: ${report.skipped}\n` +
        `Текущий стрик: ${report.streakCurrent} дней\n` +
        (report.topCategory ? `Главный результат недели: «${report.topCategory}»\n` : '') +
        (report.focusNextWeek ? `Фокус следующей недели: «${report.focusNextWeek}»\n` : '') +
        `\n+${WEEKLY_REPORT_XP} XP за то, что ты не бросил(а). Ты молодец — продолжай в том же духе! 💪`;

      await sendMessage(user.tg_id, text);
    }
  }, { timezone: 'Europe/Moscow' });

  console.log('[cron] weeklyReport запланирован на понедельник 10:00 Europe/Moscow');
}

module.exports = { start };
