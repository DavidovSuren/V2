function isTierActive(user, tier) {
  if (user.subscription_tier !== tier) return false;
  if (!user.subscription_expires_at) return false;
  return new Date(user.subscription_expires_at) > new Date();
}

function isPremiumActive(user) {
  return isTierActive(user, 'premium888');
}

function isAnySubscriptionActive(user) {
  return isTierActive(user, 'plus369') || isTierActive(user, 'premium888');
}

module.exports = { isPremiumActive, isAnySubscriptionActive };
