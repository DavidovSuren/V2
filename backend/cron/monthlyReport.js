const cron = require('node-cron');
const db = require('../db');
const { monthlyReportFor } = require('../lib/reports');
const { MONTHLY_REPORT_XP } = require('../lib/leveling');
const { sendMessage } = require('../lib/telegram');

// 1-го числа каждого месяца в 11:00 по московскому времени — полный отчёт
// с динамикой настроения + бонус XP.
function start() {
  cron.schedule('0 11 1 * *', async () => {
    const users = db.prepare('SELECT id, tg_id FROM users WHERE gender IS NOT NULL').all();

    for (const user of users) {
      const report = monthlyReportFor(user.id);
      db.prepare('UPDATE users SET xp = xp + ? WHERE id = ?').run(MONTHLY_REPORT_XP, user.id);

      const moodLine = report.moodTrend.length
        ? report.moodTrend.map(m => `нед.${m.week}: ${m.avgMood}/5`).join('  ')
        : 'пока недостаточно записей в дневнике';

      const text = `🗓 Отчёт за месяц\n\n` +
        `Выполнено заданий: ${report.completed}\n` +
        `Пропущено: ${report.skipped}\n` +
        `Лучший стрик: ${report.streakBest} дней\n` +
        `Динамика настроения: ${moodLine}\n\n` +
        `Где-то не дожал(а) — и это нормально. Ты всё равно на 30 дней ближе к своей Version 2.0. ` +
        `+${MONTHLY_REPORT_XP} XP за этот месяц! 🚀`;

      await sendMessage(user.tg_id, text);
    }
  }, { timezone: 'Europe/Moscow' });

  console.log('[cron] monthlyReport запланирован на 1-е число 11:00 Europe/Moscow');
}

module.exports = { start };
