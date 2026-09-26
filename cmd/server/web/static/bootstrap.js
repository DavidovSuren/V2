// Единственный fetch() во всём приложении: на первом заходе передаём
// Telegram.WebApp.initData на сервер, чтобы завести сессионную cookie.
// Дальше вся навигация — обычные ссылки и формы, без единого fetch.
(function () {
  var tg = window.Telegram && window.Telegram.WebApp;
  var target = document.getElementById('bootstrap-target');

  // После появления сессии перезагружаем только "/" — страница ошибки
  // приходит ответом на POST /onboarding, и reload предложил бы
  // переотправить форму. Там сессия уже есть: достаточно нажать "Начать".
  function bootstrap(body) {
    fetch('/auth/bootstrap', { method: 'POST', body: body })
      .then(function (r) { if (r.ok && location.pathname === '/') location.reload(); });
  }

  // Вне Telegram (обычный браузер) initData нет. В dev-режиме сервер
  // помечает #bootstrap-target, и мы заводим тестовую сессию с id, который
  // хранится в браузере, — при следующем заходе это тот же пользователь.
  if (target && target.dataset.devAuth && !(tg && tg.initData)) {
    var id;
    try { id = localStorage.getItem('v2_dev_id'); } catch (e) { /* приватный режим */ }
    if (!id) {
      id = 'browser-' + Math.random().toString(36).slice(2, 10);
      try { localStorage.setItem('v2_dev_id', id); } catch (e) { /* не сохранится — не страшно */ }
    }
    bootstrap('debug:' + id);
  }

  if (!tg) return;

  tg.ready();
  tg.expand();

  // У приложения своя тема (см. скрипт в <head>): шапку и фон Telegram
  // подстраиваем под --bg, а не наоборот.
  var bg = getComputedStyle(document.documentElement).getPropertyValue('--bg').trim();
  if (bg) {
    try { tg.setHeaderColor(bg); tg.setBackgroundColor(bg); } catch (e) { /* старый клиент */ }
  }

  // #bootstrap-target существует только в разметке приветственного экрана
  // (не авторизован) — на остальных страницах сессия уже есть и слать
  // initData незачем.
  if (target && tg.initData) {
    bootstrap(tg.initData);
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

// Ссылки t.me (например, «Поделиться») внутри Telegram открываем через
// openTelegramLink — иначе Mini App уйдёт на страницу во встроенном браузере.
document.querySelectorAll('[data-tg-link]').forEach(function (link) {
  link.addEventListener('click', function (e) {
    var tg = window.Telegram && window.Telegram.WebApp;
    if (tg && tg.openTelegramLink && tg.initData) {
      e.preventDefault();
      tg.openTelegramLink(link.href);
    }
  });
});
