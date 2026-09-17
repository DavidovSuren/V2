const { QUESTIONS, CATEGORIES } = require('./questions');

// Переводит ответы анкеты в вес приоритета (1..5) на каждую из 8 категорий.
// Идея: чем слабее человек оценивает себя в категории, тем выше приоритет
// (weight) — такие категории чаще и раньше попадают в его план на 365 дней.
//
// - 'scale' вопросы (1–10) дают числовую оценку напрямую.
// - 'single' диагностические вопросы: варианты в questions.js упорядочены
//   от "лучшего" состояния (index 0) к "худшему" (последний index), поэтому
//   индекс варианта конвертируется в ту же шкалу 1–10.
// - 'multi' и 'text' в подсчёт веса не идут — это просто контекст.
function computeCategoryWeights(answersByQuestionId) {
  const scoresByCategory = {};
  for (const cat of CATEGORIES) scoresByCategory[cat] = [];

  for (const q of QUESTIONS) {
    if (!q.diagnostic || !q.cat) continue;
    const answer = answersByQuestionId[q.id];
    if (answer === undefined || answer === null || answer === '') continue;

    let score = null;
    if (q.type === 'scale') {
      const n = Number(answer);
      if (!Number.isNaN(n)) score = Math.min(10, Math.max(1, n));
    } else if (q.type === 'single') {
      const idx = q.options.indexOf(answer);
      if (idx >= 0 && q.options.length > 1) {
        score = 10 - (idx / (q.options.length - 1)) * 9;
      }
    }
    if (score !== null) scoresByCategory[q.cat].push(score);
  }

  const weights = {};
  for (const cat of CATEGORIES) {
    const scores = scoresByCategory[cat];
    if (!scores.length) {
      weights[cat] = 3; // нейтральный приоритет, если нет диагностических ответов
      continue;
    }
    const avg = scores.reduce((a, b) => a + b, 0) / scores.length;
    // avg=10 (всё отлично) -> weight=1 ; avg=1 (всё плохо) -> weight=5
    weights[cat] = Math.min(5, Math.max(1, 1 + ((10 - avg) / 9) * 4));
  }
  return weights;
}

// Weighted round robin: строит порядок из 365 заданий так, чтобы категории
// с более высоким весом встречались раньше и чаще, но КАЖДОЕ задание банка
// было использовано ровно один раз к 365-му дню (весь банк расходуется).
function buildSchedule(taskBank, categoryWeights) {
  const byCategory = {};
  for (const task of taskBank) {
    if (!byCategory[task.category]) byCategory[task.category] = [];
    byCategory[task.category].push(task);
  }

  const categories = Object.keys(byCategory);
  const pointer = {};
  const credit = {};
  const weight = {};
  categories.forEach(c => {
    pointer[c] = 0;
    credit[c] = 0;
    weight[c] = categoryWeights[c] || 3;
  });

  const schedule = [];
  let remaining = taskBank.length;

  while (remaining > 0) {
    const active = categories.filter(c => pointer[c] < byCategory[c].length);
    const totalActiveWeight = active.reduce((sum, c) => sum + weight[c], 0);
    active.forEach(c => { credit[c] += weight[c]; });

    let chosen = active[0];
    for (const c of active) if (credit[c] > credit[chosen]) chosen = c;

    schedule.push(byCategory[chosen][pointer[chosen]]);
    pointer[chosen] += 1;
    credit[chosen] -= totalActiveWeight;
    remaining -= 1;
  }

  return schedule;
}

module.exports = { computeCategoryWeights, buildSchedule };
