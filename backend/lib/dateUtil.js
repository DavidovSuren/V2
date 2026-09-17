// Все "календарные дни" в приложении считаются по московскому времени —
// это важно для "одно действие в день" и для времени напоминания (15:15 МСК).
const MSK_FORMATTER = new Intl.DateTimeFormat('en-CA', {
  timeZone: 'Europe/Moscow',
  year: 'numeric', month: '2-digit', day: '2-digit'
});

function todayMoscow() {
  return MSK_FORMATTER.format(new Date()); // 'YYYY-MM-DD'
}

function isYesterday(dateStr, todayStr) {
  if (!dateStr) return false;
  const d = new Date(dateStr + 'T00:00:00Z');
  const t = new Date(todayStr + 'T00:00:00Z');
  const diffDays = Math.round((t - d) / 86400000);
  return diffDays === 1;
}

module.exports = { todayMoscow, isYesterday };
