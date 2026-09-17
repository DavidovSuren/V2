// Express 4 не ловит отклонённые промисы из async-хендлеров/миддлварей сама —
// без этой обёртки ошибка БД просто "теряется" и запрос зависает.
function wrap(fn) {
  return (req, res, next) => {
    Promise.resolve(fn(req, res, next)).catch(next);
  };
}

module.exports = { wrap };
