const db = require('../db');
const { STREAK_MILESTONES, LEVEL_MILESTONES } = require('./leveling');

// Проверяет стрик и уровень пользователя и открывает новые бейджи.
// Возвращает список кодов, которые были открыты именно сейчас (для тостов).
async function checkAndUnlock(user) {
  const unlockedNow = [];
  const now = new Date().toISOString();

  const milestones = [
    ...STREAK_MILESTONES.map(days => ({ code: `${days}d`, reached: user.streak_current >= days })),
    ...LEVEL_MILESTONES.map(lvl => ({ code: `lvl${lvl}`, reached: user.level >= lvl }))
  ];

  for (const { code, reached } of milestones) {
    if (!reached) continue;
    const has = await db.get('SELECT 1 FROM achievements WHERE user_id = ? AND code = ?', [user.id, code]);
    if (has) continue;
    await db.run(
      'INSERT INTO achievements (user_id, code, unlocked_at) VALUES (?, ?, ?) ON CONFLICT (user_id, code) DO NOTHING',
      [user.id, code, now]
    );
    unlockedNow.push(code);
  }

  return unlockedNow;
}

const ACHIEVEMENT_META = {
  '7d': { icon: '🔥', name: '7 ДНЕЙ', desc: '7 дней подряд без пропусков' },
  '30d': { icon: '💪', name: '30 ДНЕЙ', desc: '30 дней подряд без пропусков' },
  '180d': { icon: '🚀', name: '180 ДНЕЙ', desc: '180 дней подряд без пропусков' },
  '365d': { icon: '👑', name: '365 ДНЕЙ', desc: 'Целый год без пропусков' },
  'lvl25': { icon: '⭐', name: 'УРОВЕНЬ 25', desc: 'Четверть пути пройдена' },
  'lvl50': { icon: '🌟', name: 'УРОВЕНЬ 50', desc: 'Половина пути пройдена' },
  'lvl75': { icon: '✨', name: 'УРОВЕНЬ 75', desc: 'Три четверти пути' },
  'lvl100': { icon: '💎', name: 'УРОВЕНЬ 100', desc: 'Version 2.0 полностью собрана' }
};

module.exports = { checkAndUnlock, ACHIEVEMENT_META };
