// Момент «Сделал»: короткое конфетти (1,2 с, без библиотек) и вибрация
// Telegram на главной после сохранения дня (?done=1). Уважаем
// prefers-reduced-motion — тогда только вибрация.
(function () {
  var root = document.querySelector('[data-confetti]');
  if (!root) return;

  var tg = window.Telegram && window.Telegram.WebApp;
  try { tg && tg.HapticFeedback && tg.HapticFeedback.notificationOccurred('success'); } catch (e) { /* старый клиент */ }

  // Убираем ?done=1, чтобы обновление страницы не повторяло праздник.
  try {
    var url = new URL(location.href);
    url.searchParams.delete('done');
    history.replaceState(null, '', url.pathname + (url.search || '') + url.hash);
  } catch (e) { /* не страшно */ }

  if (window.matchMedia && matchMedia('(prefers-reduced-motion: reduce)').matches) return;

  var colors = ['var(--gold)', 'var(--flame)', 'var(--success)', 'var(--gold-text)', 'var(--violet)'];
  for (var i = 0; i < 36; i++) {
    var p = document.createElement('i');
    p.style.left = (Math.random() * 100) + 'vw';
    p.style.background = colors[i % colors.length];
    p.style.animationDelay = (Math.random() * 0.25) + 's';
    p.style.setProperty('--drift', (Math.random() * 80 - 40) + 'px');
    p.style.setProperty('--spin', (Math.random() * 540 - 270) + 'deg');
    root.appendChild(p);
  }
  setTimeout(function () { root.remove(); }, 1600);
})();
