const TOTAL_TASKS = 365;
const LEVEL_MILESTONES = [25, 50, 75, 100];
const STREAK_MILESTONES = [7, 30, 180, 365];

// Прогресс и уровень считаются только от количества реально выполненных
// заданий — 365/365 = 100% = уровень 100 (и алмаз). Один уровень ≈ 3.65 задания.
function progressFromCompleted(completedCount) {
  const progressPct = Math.min(100, (completedCount / TOTAL_TASKS) * 100);
  const level = Math.min(100, Math.floor(progressPct));
  return { progressPct, level };
}

// XP — отдельная "игровая" валюта для мотивации, на level/progress не влияет.
// Базовое начисление за задание + бонус за каждую полную неделю стрика.
function xpForTaskCompletion(streakCurrentAfter) {
  const base = 10;
  const streakBonus = Math.min(20, Math.floor(streakCurrentAfter / 7) * 5);
  return base + streakBonus;
}

const WEEKLY_REPORT_XP = 25;
const MONTHLY_REPORT_XP = 60;
const QUIZ_COMPLETE_XP = 50;

function categoryProgressPct(doneInCategory, totalInCategoryBank) {
  if (!totalInCategoryBank) return 0;
  return Math.round((doneInCategory / totalInCategoryBank) * 100);
}

module.exports = {
  TOTAL_TASKS,
  LEVEL_MILESTONES,
  STREAK_MILESTONES,
  progressFromCompleted,
  xpForTaskCompletion,
  WEEKLY_REPORT_XP,
  MONTHLY_REPORT_XP,
  QUIZ_COMPLETE_XP,
  categoryProgressPct
};
