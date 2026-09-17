const fs = require('fs');
const path = require('path');

const male = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'data', 'tasks_male.json'), 'utf8'));
const female = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'data', 'tasks_female.json'), 'utf8'));

const banks = { male, female };

function bankFor(gender) {
  const bank = banks[gender];
  if (!bank) throw new Error(`Неизвестный пол: ${gender}`);
  return bank;
}

// Сколько всего заданий в каждой категории — нужно для процента прогресса
// по направлениям (done_in_category / total_in_category_bank).
function categoryTotals(gender) {
  const totals = {};
  for (const task of bankFor(gender)) {
    totals[task.category] = (totals[task.category] || 0) + 1;
  }
  return totals;
}

module.exports = { bankFor, categoryTotals };
