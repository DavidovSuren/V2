const express = require('express');
const db = require('../db');
const { identify, requireUser } = require('../lib/telegramAuth');
const { PRICES, PREMIUM_AGENT_COMMISSION_PCT, discountPctForReferrer, priceAfterDiscount } = require('../lib/referrals');

const router = express.Router();

// Оплата остаётся симуляцией (как и в исходном MVP — реальный платёжный
// шлюз не подключён), но скидки и комиссии считаются по-настоящему и
// сохраняются в БД.
router.post('/subscribe', identify, requireUser, (req, res) => {
  const { tier } = req.body || {};
  if (!PRICES[tier]) return res.status(400).json({ error: 'Неизвестный тариф' });

  const user = req.user;
  const basePrice = PRICES[tier];

  let discountPct = 0;
  let referrerCommissionUserId = null;
  let commissionAmount = 0;

  if (user.referred_by_user_id) {
    if (user.referred_by_code_type === 'premium_agent') {
      discountPct = 0; // по премиум-агентскому коду приглашённый платит полную цену
    } else {
      const priorPaidReferrals = db.prepare(`
        SELECT COUNT(DISTINCT r.referred_id) as cnt
        FROM referrals r
        JOIN subscription_payments sp ON sp.user_id = r.referred_id
        WHERE r.referrer_id = ?
      `).get(user.referred_by_user_id).cnt;
      discountPct = discountPctForReferrer(priorPaidReferrals);
    }
  }

  const pricePaid = priceAfterDiscount(basePrice, discountPct);

  if (user.referred_by_user_id && user.referred_by_code_type === 'premium_agent') {
    referrerCommissionUserId = user.referred_by_user_id;
    commissionAmount = Math.round(pricePaid * PREMIUM_AGENT_COMMISSION_PCT / 100);
  }

  const now = new Date();
  const expires = new Date(now);
  expires.setMonth(expires.getMonth() + 1);

  const tx = db.transaction(() => {
    db.prepare(`
      INSERT INTO subscription_payments
        (user_id, tier, base_price, discount_pct, price_paid, referrer_commission_user_id, commission_amount, paid_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `).run(user.id, tier, basePrice, discountPct, pricePaid, referrerCommissionUserId, commissionAmount, now.toISOString());

    db.prepare('UPDATE users SET subscription_tier = ?, subscription_expires_at = ? WHERE id = ?')
      .run(tier, expires.toISOString(), user.id);

    if (referrerCommissionUserId) {
      db.prepare(`
        INSERT INTO wallet_transactions (user_id, amount, source_user_id, note, created_at)
        VALUES (?, ?, ?, ?, ?)
      `).run(referrerCommissionUserId, commissionAmount, user.id, `Комиссия с оплаты ${tier}`, now.toISOString());
    }
  });
  tx();

  res.json({ ok: true, tier, basePrice, discountPct, pricePaid, expiresAt: expires.toISOString() });
});

// Скидка, которую увидит пользователь ДО оплаты — для экрана "Профиль".
router.get('/subscribe/quote', identify, requireUser, (req, res) => {
  const user = req.user;
  const quote = {};
  for (const tier of Object.keys(PRICES)) {
    let discountPct = 0;
    if (user.referred_by_user_id) {
      if (user.referred_by_code_type === 'premium_agent') {
        discountPct = 0;
      } else {
        const priorPaidReferrals = db.prepare(`
          SELECT COUNT(DISTINCT r.referred_id) as cnt
          FROM referrals r
          JOIN subscription_payments sp ON sp.user_id = r.referred_id
          WHERE r.referrer_id = ?
        `).get(user.referred_by_user_id).cnt;
        discountPct = discountPctForReferrer(priorPaidReferrals);
      }
    }
    quote[tier] = {
      basePrice: PRICES[tier],
      discountPct,
      pricePaid: priceAfterDiscount(PRICES[tier], discountPct)
    };
  }
  res.json({ quote, currentTier: user.subscription_tier, currentExpiresAt: user.subscription_expires_at });
});

router.get('/wallet', identify, requireUser, (req, res) => {
  const user = req.user;
  if (!user.premium_agent_code) {
    return res.status(403).json({ error: 'Кошелёк доступен только держателям премиум-агентского кода' });
  }

  const balance = db.prepare('SELECT COALESCE(SUM(amount), 0) as total FROM wallet_transactions WHERE user_id = ?')
    .get(user.id).total;

  const payingReferrals = db.prepare(`
    SELECT COUNT(DISTINCT user_id) as cnt FROM subscription_payments WHERE referrer_commission_user_id = ?
  `).get(user.id).cnt;

  const history = db.prepare(`
    SELECT amount, note, created_at FROM wallet_transactions WHERE user_id = ? ORDER BY created_at DESC LIMIT 50
  `).all(user.id);

  res.json({ code: user.premium_agent_code, balance, payingReferrals, history });
});

router.post('/wallet/withdraw', identify, requireUser, (req, res) => {
  if (!req.user.premium_agent_code) {
    return res.status(403).json({ error: 'Кошелёк доступен только держателям премиум-агентского кода' });
  }
  res.json({ ok: false, message: 'Вывод средств скоро появится — сейчас баланс копится в приложении.' });
});

module.exports = router;
