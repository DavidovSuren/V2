// Окно оплаты Telegram для инвойса, созданного сервером (createInvoiceLink).
// Подписку активирует сервер по successful_payment из вебхука — здесь только
// переход на страницу тарифов с понятным статусом.
(function () {
  var el = document.getElementById('invoice');
  var tg = window.Telegram && window.Telegram.WebApp;
  if (!el) return;
  var link = el.dataset.invoiceLink;

  function open() {
    if (!tg || !tg.openInvoice) {
      location.href = link; // вне Telegram — ссылка на инвойс
      return;
    }
    tg.openInvoice(link, function (status) {
      if (status === 'paid') location.href = '/plans?paid=1';
      else if (status === 'pending') location.href = '/plans?pay=pending';
      else location.href = '/plans?pay=' + (status === 'cancelled' ? 'cancelled' : 'failed');
    });
  }

  document.getElementById('pay-button').addEventListener('click', open);
  open();
})();
