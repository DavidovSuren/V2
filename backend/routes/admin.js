const express = require('express');
const db = require('../db');
const { identify } = require('../lib/telegramAuth');
const { generateCode } = require('../lib/codes');

const router = express.Router();

function requireAdmin(req, res, next) {
  const adminId = process.env.ADMIN_TG_ID;
  if (!adminId || req.tg.id !== adminId) {
    return res.status(403).json({ error: 'Доступно только владельцу приложения' });
  }
  next();
}

// Премиум-агентские коды выдаёт только владелец — держатель зарабатывает
// 50% с каждой оплаты своих приглашённых, а те платят полную цену.
router.post('/admin/grant-premium-agent', identify, requireAdmin, (req, res) => {
  const { username, tgId } = req.body || {};
  if (!username && !tgId) return res.status(400).json({ error: 'Укажи username или tgId пользователя' });

  const target = tgId
    ? db.prepare('SELECT * FROM users WHERE tg_id = ?').get(String(tgId))
    : db.prepare('SELECT * FROM users WHERE username = ?').get(String(username).replace(/^@/, ''));

  if (!target) return res.status(404).json({ error: 'Пользователь не найден' });
  if (target.premium_agent_code) {
    return res.json({ ok: true, code: target.premium_agent_code, alreadyGranted: true });
  }

  let code = generateCode(8);
  while (db.prepare('SELECT 1 FROM users WHERE premium_agent_code = ?').get(code)) {
    code = generateCode(8);
  }

  db.prepare('UPDATE users SET premium_agent_code = ? WHERE id = ?').run(code, target.id);
  res.json({ ok: true, code });
});

module.exports = router;
