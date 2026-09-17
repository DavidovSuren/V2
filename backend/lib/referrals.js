const PRICES = { plus369: 369, premium888: 888 };
const PREMIUM_AGENT_COMMISSION_PCT = 50;

// Скидка для НОВОГО приглашённого зависит от того, сколько людей его
// пригласивший уже успешно привёл к оплате (paidReferralsCount — на момент
// оплаты этого нового человека, ДО текущей оплаты).
// Примеры из ТЗ: 1–3 оплативших приглашённых -> 20%; >3 -> 35%; пригласил
// 10 -> 11-й уже получает 50% (т.е. порог считается по факту "10 уже есть").
function discountPctForReferrer(paidReferralsCount) {
  if (paidReferralsCount >= 10) return 50;
  if (paidReferralsCount >= 4) return 35;
  return 20;
}

function priceAfterDiscount(basePrice, discountPct) {
  return Math.round(basePrice * (1 - discountPct / 100));
}

module.exports = { PRICES, PREMIUM_AGENT_COMMISSION_PCT, discountPctForReferrer, priceAfterDiscount };
