// Анкета: для вариантов и шкалы 1–10 — сразу следующий вопрос после выбора
// (текстовые вопросы — кнопкой «Далее»).
(function () {
  var form = document.querySelector('form[data-autosubmit]');
  if (!form) return;
  form.addEventListener('change', function (e) {
    if (e.target && e.target.type === 'radio') {
      var tg = window.Telegram && window.Telegram.WebApp;
      try { tg && tg.HapticFeedback && tg.HapticFeedback.selectionChanged(); } catch (err) { /* старый клиент */ }
      setTimeout(function () { form.submit(); }, 150); // дать увидеть выбор
    }
  });
})();
