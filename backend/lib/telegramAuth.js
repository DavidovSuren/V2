const crypto = require('crypto');
const db = require('../db');

// Проверка initData по алгоритму Telegram:
// https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
function verifyInitData(initData, botToken) {
  if (!initData || !botToken) return null;
  const params = new URLSearchParams(initData);
  const hash = params.get('hash');
  if (!hash) return null;
  params.delete('hash');

  const pairs = [];
  for (const [key, value] of params.entries()) pairs.push(`${key}=${value}`);
  pairs.sort();
  const dataCheckString = pairs.join('\n');

  const secretKey = crypto.createHmac('sha256', 'WebAppData').update(botToken).digest();
  const computedHash = crypto.createHmac('sha256', secretKey).update(dataCheckString).digest('hex');

  if (computedHash !== hash) return null;

  const userJson = params.get('user');
  if (!userJson) return null;
  try {
    const user = JSON.parse(userJson);
    return { id: String(user.id), username: user.username || null, name: user.first_name || 'Друг' };
  } catch {
    return null;
  }
}

// Определяет, кто стучится в API. В проде — только настоящий initData.
// В деве (DEV_ALLOW_FAKE_AUTH=true) можно передать X-Debug-Tg-Id и т.д.,
// чтобы тестировать бэкенд без Telegram.
function identify(req, res, next) {
  const botToken = process.env.BOT_TOKEN;
  const authHeader = req.headers['authorization'] || '';

  if (authHeader.startsWith('tma ')) {
    const initData = authHeader.slice(4);
    const identity = verifyInitData(initData, botToken);
    if (identity) {
      req.tg = identity;
      return next();
    }
  }

  if (process.env.DEV_ALLOW_FAKE_AUTH === 'true' && req.headers['x-debug-tg-id']) {
    // X-Debug-Name кодируется на клиенте (HTTP-заголовки не поддерживают не-ASCII напрямую).
    let name = 'Тестовый пользователь';
    if (req.headers['x-debug-name']) {
      try { name = decodeURIComponent(req.headers['x-debug-name']); } catch { /* оставляем дефолт */ }
    }
    req.tg = {
      id: String(req.headers['x-debug-tg-id']),
      username: req.headers['x-debug-username'] || null,
      name
    };
    return next();
  }

  return res.status(401).json({ error: 'Не удалось подтвердить Telegram-пользователя' });
}

// Требует, чтобы пользователь уже был зарегистрирован (прошёл онбординг).
function requireUser(req, res, next) {
  let row = db.prepare('SELECT * FROM users WHERE tg_id = ?').get(req.tg.id);
  if (!row) return res.status(404).json({ error: 'Пользователь ещё не зарегистрирован' });

  if (req.tg.username && req.tg.username !== row.username) {
    db.prepare('UPDATE users SET username = ? WHERE id = ?').run(req.tg.username, row.id);
    row = db.prepare('SELECT * FROM users WHERE id = ?').get(row.id);
  }

  req.user = row;
  next();
}

module.exports = { verifyInitData, identify, requireUser };
