// Кнопка «Начать» неактивна, пока не отмечено согласие с соглашением
// (сервер тоже проверяет — без галочки пользователь не создаётся).
(function () {
  document.querySelectorAll('[data-terms-form]').forEach(function (form) {
    var check = form.querySelector('[data-terms-check]');
    var submit = form.querySelector('[data-terms-submit]');
    if (!check || !submit) return;
    function sync() { submit.disabled = !check.checked; }
    check.addEventListener('change', sync);
    sync();
  });
})();
