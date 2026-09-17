const express = require('express');
const db = require('../db');
const { identify, requireUser } = require('../lib/telegramAuth');
const { QUESTIONS, CATEGORIES } = require('../lib/questions');
const { computeCategoryWeights, buildSchedule } = require('../lib/scheduler');
const { bankFor } = require('../lib/taskBank');
const { generateCode } = require('../lib/codes');
const { progressFromCompleted, QUIZ_COMPLETE_XP } = require('../lib/leveling');

const router = express.Router();

router.get('/quiz/questions', (req, res) => {
  res.json({ questions: QUESTIONS });
});

// Кто я — используется фронтом при старте, чтобы решить, на какой экран
// попасть (приветствие / анкета / главная).
router.get('/me', identify, (req, res) => {
  const user = db.prepare('SELECT * FROM users WHERE tg_id = ?').get(req.tg.id);
  if (!user) return res.json({ registered: false });

  const hasQuiz = db.prepare('SELECT 1 FROM user_schedule WHERE user_id = ?').get(user.id);
  res.json({
    registered: true,
    quizDone: !!hasQuiz,
    name: user.name,
    gender: user.gender,
    ageGroup: user.age_group,
    level: user.level,
    xp: user.xp,
    progressPct: progressFromCompleted(user.completed_count).progressPct,
    subscriptionTier: user.subscription_tier,
    referralCode: user.referral_code,
    hasPremiumAgentCode: !!user.premium_agent_code
  });
});

router.post('/onboarding', identify, (req, res) => {
  const { name, ageGroup, photos, refCode } = req.body || {};
  if (!name || !ageGroup) {
    return res.status(400).json({ error: 'Нужны имя и возрастная группа' });
  }

  let user = db.prepare('SELECT * FROM users WHERE tg_id = ?').get(req.tg.id);

  if (user) {
    db.prepare('UPDATE users SET name = ?, age_group = ?, photos_json = ? WHERE id = ?')
      .run(name, ageGroup, JSON.stringify(photos || []), user.id);
    user = db.prepare('SELECT * FROM users WHERE id = ?').get(user.id);
    return res.json({ ok: true, referralCode: user.referral_code });
  }

  let referredBy = null;
  let referredByCodeType = null;
  if (refCode) {
    const referrer = db.prepare(
      'SELECT * FROM users WHERE referral_code = ? OR premium_agent_code = ?'
    ).get(refCode, refCode);
    if (referrer) {
      referredBy = referrer.id;
      referredByCodeType = referrer.premium_agent_code === refCode ? 'premium_agent' : 'normal';
    }
  }

  let referralCode = generateCode(7);
  while (db.prepare('SELECT 1 FROM users WHERE referral_code = ?').get(referralCode)) {
    referralCode = generateCode(7);
  }

  const insert = db.prepare(`
    INSERT INTO users (tg_id, username, name, age_group, photos_json, created_at,
                        referral_code, referred_by_user_id, referred_by_code_type)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
  `);
  const info = insert.run(
    req.tg.id, req.tg.username, name, ageGroup, JSON.stringify(photos || []),
    new Date().toISOString(), referralCode, referredBy, referredByCodeType
  );

  if (referredBy) {
    db.prepare(`
      INSERT INTO referrals (referrer_id, referred_id, code_type, created_at)
      VALUES (?, ?, ?, ?)
    `).run(referredBy, info.lastInsertRowid, referredByCodeType, new Date().toISOString());
  }

  res.json({ ok: true, referralCode });
});

router.post('/quiz', identify, requireUser, (req, res) => {
  const { answers } = req.body || {};
  if (!answers || typeof answers !== 'object') {
    return res.status(400).json({ error: 'Нужны ответы анкеты' });
  }

  for (const q of QUESTIONS) {
    const a = answers[q.id];
    if (a === undefined || a === null || a === '' || (Array.isArray(a) && a.length === 0)) {
      return res.status(400).json({ error: `Не отвечен вопрос ${q.id}` });
    }
  }

  const gender = answers[0];
  if (gender !== 'male' && gender !== 'female') {
    return res.status(400).json({ error: 'Некорректный ответ на вопрос о поле' });
  }

  const user = req.user;

  const insertAnswer = db.prepare(`
    INSERT INTO quiz_answers (user_id, question_id, answer_json) VALUES (?, ?, ?)
    ON CONFLICT(user_id, question_id) DO UPDATE SET answer_json = excluded.answer_json
  `);
  const diagnosticAnswers = {};
  for (const q of QUESTIONS) {
    if (q.id === 0) continue;
    insertAnswer.run(user.id, q.id, JSON.stringify(answers[q.id]));
    diagnosticAnswers[q.id] = answers[q.id];
  }

  const weights = computeCategoryWeights(diagnosticAnswers);
  const insertWeight = db.prepare(`
    INSERT INTO category_weights (user_id, category, weight) VALUES (?, ?, ?)
    ON CONFLICT(user_id, category) DO UPDATE SET weight = excluded.weight
  `);
  for (const cat of CATEGORIES) insertWeight.run(user.id, cat, weights[cat]);

  const bank = bankFor(gender);
  const schedule = buildSchedule(bank, weights);

  const insertSchedule = db.prepare(`
    INSERT INTO user_schedule (user_id, day_index, task_id, category, task_text, task_why, status)
    VALUES (?, ?, ?, ?, ?, ?, 'pending')
  `);
  const tx = db.transaction(() => {
    schedule.forEach((task, i) => {
      insertSchedule.run(user.id, i, task.id, task.category, task.text, task.why);
    });
    db.prepare('UPDATE users SET gender = ?, day_index = 0, xp = xp + ? WHERE id = ?')
      .run(gender, QUIZ_COMPLETE_XP, user.id);
  });
  tx();

  // Топ-3 направления с наибольшим приоритетом — показываем пользователю
  // сразу после анкеты как персональный вывод ("на что мы сделали акцент").
  const focusAreas = CATEGORIES
    .map(cat => ({ category: cat, weight: weights[cat] }))
    .sort((a, b) => b.weight - a.weight)
    .slice(0, 3)
    .map(f => f.category);

  const today = schedule[0];
  res.json({
    ok: true,
    xpGain: QUIZ_COMPLETE_XP,
    focusAreas,
    today: { dayIndex: 0, category: today.category, text: today.text, why: today.why }
  });
});

module.exports = router;
