// Тонкая обёртка над Telegram Bot API. Если BOT_TOKEN не задан в .env,
// все функции тихо ничего не делают (и логируют это один раз) — сервер
// не должен падать из-за отсутствия токена в деве.
let warned = false;

function warnOnce() {
  if (!warned) {
    console.warn('[telegram] BOT_TOKEN не задан — push-уведомления и сообщения бота отключены.');
    warned = true;
  }
}

async function sendMessage(tgId, text) {
  const token = process.env.BOT_TOKEN;
  if (!token) {
    warnOnce();
    return { ok: false, skipped: true };
  }
  try {
    const res = await fetch(`https://api.telegram.org/bot${token}/sendMessage`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ chat_id: tgId, text, parse_mode: 'HTML' })
    });
    const data = await res.json();
    if (!data.ok) console.error('[telegram] sendMessage failed:', data.description);
    return data;
  } catch (err) {
    console.error('[telegram] sendMessage error:', err.message);
    return { ok: false, error: err.message };
  }
}

module.exports = { sendMessage };
