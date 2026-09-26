// Автоформат номера карты при вводе: "2200 1234 5678 9012".
// Сохранённую маску (•••• •••• •••• 4417) при первом вводе цифры стираем.
(function () {
  var input = document.querySelector('[data-card-input]');
  if (!input) return;
  input.addEventListener('input', function () {
    if (input.value.indexOf('•') !== -1) {
      input.value = input.value.replace(/[^0-9]/g, '').slice(-1);
    }
    var digits = input.value.replace(/\D/g, '').slice(0, 19);
    input.value = digits.replace(/(\d{4})(?=\d)/g, '$1 ');
  });
})();
