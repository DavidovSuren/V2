const express = require('express');
const db = require('../db');
const { identify, requireUser } = require('../lib/telegramAuth');
const { wrap } = require('../lib/asyncHandler');
const { isPremiumActive } = require('../lib/subscriptionUtil');

const router = express.Router();

async function attachBadges(users) {
  if (users.length === 0) return users;
  const ids = users.map(u => u.id);
  const placeholders = ids.map(() => '?').join(',');
  const rows = await db.all(
    `SELECT user_id, code FROM achievements WHERE user_id IN (${placeholders})`, ids
  );
  const byUser = {};
  rows.forEach(r => { (byUser[r.user_id] = byUser[r.user_id] || []).push(r.code); });
  return users.map(u => ({
    ...u,
    badges: byUser[u.id] || [],
    diamond: u.level >= 100
  }));
}

// Список всех пользователей приложения. Виден каждому, но уровень/бейджи/
// алмаз добавляются в ответ только тем, у кого активна подписка 888 ₽.
router.get('/community', identify, requireUser, wrap(async (req, res) => {
  const viewerPremium = isPremiumActive(req.user);

  const rows = await db.all(`
    SELECT id, name, gender, level FROM users ORDER BY level DESC, created_at ASC
  `);

  if (!viewerPremium) {
    return res.json({
      premiumView: false,
      users: rows.map(({ id, name, gender }) => ({ id, name, gender })),
      upsell: 'Оформи подписку 888 ₽/мес, чтобы видеть уровень и достижения всех участников 🔒'
    });
  }

  res.json({ premiumView: true, users: await attachBadges(rows) });
}));

// «Контакты» — только для 888: реферальная сеть (в обе стороны) +
// добавленные вручную по @username друзья.
router.get('/friends', identify, requireUser, wrap(async (req, res) => {
  if (!isPremiumActive(req.user)) {
    return res.status(403).json({ error: 'Раздел «Контакты» доступен только с подпиской 888 ₽/мес' });
  }

  const userId = req.user.id;
  const rows = await db.all(`
    SELECT DISTINCT u.id, u.name, u.gender, u.level FROM users u
    WHERE u.id IN (
      SELECT referred_id FROM referrals WHERE referrer_id = ?
      UNION
      SELECT referrer_id FROM referrals WHERE referred_id = ?
      UNION
      SELECT friend_id FROM friendships WHERE user_id = ?
    )
    ORDER BY u.level DESC
  `, [userId, userId, userId]);

  res.json({ friends: await attachBadges(rows) });
}));

router.post('/friends/add', identify, requireUser, wrap(async (req, res) => {
  const { username } = req.body || {};
  if (!username) return res.status(400).json({ error: 'Укажи @username друга' });

  const clean = String(username).replace(/^@/, '');
  const friend = await db.get('SELECT * FROM users WHERE username = ?', [clean]);
  if (!friend) return res.status(404).json({ error: 'Такой пользователь не найден в приложении' });
  if (friend.id === req.user.id) return res.status(400).json({ error: 'Нельзя добавить самого себя' });

  const now = new Date().toISOString();
  await db.run('INSERT INTO friendships (user_id, friend_id, created_at) VALUES (?, ?, ?) ON CONFLICT (user_id, friend_id) DO NOTHING',
    [req.user.id, friend.id, now]);
  await db.run('INSERT INTO friendships (user_id, friend_id, created_at) VALUES (?, ?, ?) ON CONFLICT (user_id, friend_id) DO NOTHING',
    [friend.id, req.user.id, now]);

  res.json({ ok: true, friend: { id: friend.id, name: friend.name, gender: friend.gender, level: friend.level } });
}));

module.exports = router;
