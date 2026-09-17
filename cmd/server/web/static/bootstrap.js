// Единственный fetch() во всём приложении: на первом заходе передаём
// Telegram.WebApp.initData на сервер, чтобы завести сессионную cookie.
// Дальше вся навигация — обычные ссылки и формы, без единого fetch.
(function () {
  var tg = window.Telegram && window.Telegram.WebApp;
  if (!tg) return;

  tg.ready();
  tg.expand();

  var tp = tg.themeParams || {};
  var root = document.documentElement;
  var map = {
    bg_color: '--tg-theme-bg-color',
    text_color: '--tg-theme-text-color',
    hint_color: '--tg-theme-hint-color',
    link_color: '--tg-theme-link-color',
    button_color: '--tg-theme-button-color',
    button_text_color: '--tg-theme-button-text-color',
    secondary_bg_color: '--tg-theme-secondary-bg-color'
  };
  Object.keys(map).forEach(function (key) {
    if (tp[key]) root.style.setProperty(map[key], tp[key]);
  });

  // #bootstrap-target существует только в разметке приветственного экрана
  // (не авторизован) — на остальных страницах сессия уже есть и слать
  // initData незачем.
  if (document.getElementById('bootstrap-target') && tg.initData) {
    fetch('/auth/bootstrap', { method: 'POST', body: tg.initData })
      .then(function (r) { if (r.ok) location.reload(); });
  }
})();

// Открытие/закрытие модалок на native <dialog> — без ajax.
document.querySelectorAll('[data-open-dialog]').forEach(function (btn) {
  btn.addEventListener('click', function () {
    var dlg = document.getElementById(btn.dataset.openDialog);
    if (dlg) dlg.showModal();
  });
});
document.querySelectorAll('[data-close-dialog]').forEach(function (btn) {
  btn.addEventListener('click', function () {
    btn.closest('dialog').close();
  });
});

// Автопоказ celebrate-модалки, если сервер пометил её в разметке.
var celebrateDlg = document.getElementById('modal-celebrate');
if (celebrateDlg && celebrateDlg.dataset.autoshow === '1') {
  celebrateDlg.showModal();
}
