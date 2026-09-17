// ========== Telegram WebApp ==========
const tg = window.Telegram?.WebApp;
const BOT_USERNAME = window.V2_BOT_USERNAME || '';

if (tg) {
  tg.ready();
  tg.expand();
  const tp = tg.themeParams || {};
  const root = document.documentElement;
  if (tp.bg_color) root.style.setProperty('--tg-theme-bg-color', tp.bg_color);
  if (tp.text_color) root.style.setProperty('--tg-theme-text-color', tp.text_color);
  if (tp.hint_color) root.style.setProperty('--tg-theme-hint-color', tp.hint_color);
  if (tp.link_color) root.style.setProperty('--tg-theme-link-color', tp.link_color);
  if (tp.button_color) root.style.setProperty('--tg-theme-button-color', tp.button_color);
  if (tp.button_text_color) root.style.setProperty('--tg-theme-button-text-color', tp.button_text_color);
  if (tp.secondary_bg_color) root.style.setProperty('--tg-theme-secondary-bg-color', tp.secondary_bg_color);
}

// ========== Состояние ==========
const state = {
  me: null,
  quizQuestions: [],
  quizIndex: 0,
  quizAnswers: {},
  photos: [],
  selectedEmoji: null,
  today: null
};

const CATEGORY_ICON = {
  'Внешность': '💄', 'Стиль': '👔', 'Тело': '💪', 'Деньги': '💰',
  'Отношения': '❤️', 'Окружение': '🌱', 'Карьера': '📈', 'Образ жизни': '🌤'
};

// ========== Утилиты UI ==========
function showToast(text) {
  const t = document.getElementById('toast');
  t.textContent = text;
  t.classList.add('show');
  setTimeout(() => t.classList.remove('show'), 2800);
}
function openModal(id) { document.getElementById(id).classList.add('show'); }
function closeModal(id) { document.getElementById(id).classList.remove('show'); }

function celebrate(icon, title, text) {
  document.getElementById('celebrate-icon').textContent = icon;
  document.getElementById('celebrate-title').textContent = title;
  document.getElementById('celebrate-text').textContent = text;
  openModal('modal-celebrate');
}

async function guarded(fn) {
  try {
    return await fn();
  } catch (err) {
    showToast(err.message || 'Что-то пошло не так');
    throw err;
  }
}

// ========== Роутинг ==========
async function showScreen(name) {
  document.querySelectorAll('.screen').forEach(s => s.classList.remove('active'));
  const screen = document.getElementById('screen-' + name);
  if (screen) screen.classList.add('active');

  const nav = document.getElementById('bottom-nav');
  const showNav = ['home', 'progress', 'diary', 'community', 'achievements', 'profile'].includes(name);
  nav.style.display = showNav ? 'flex' : 'none';
  document.querySelectorAll('.nav-item').forEach(item => {
    item.classList.toggle('active', item.dataset.screen === name);
  });

  if (name === 'quiz') renderQuestion();
  if (name === 'home') await loadHome();
  if (name === 'progress') await loadProgress();
  if (name === 'diary') await loadDiaryExtras();
  if (name === 'community') {
    communityTab = 'all';
    document.querySelectorAll('#community-tabs button').forEach(b => b.classList.toggle('active', b.dataset.tab === 'all'));
    await loadCommunity('all');
  }
  if (name === 'achievements') await loadAchievements();
  if (name === 'profile') await loadProfile();
  if (name === 'report') await loadReport();
  if (name === 'wallet') await loadWallet();
}

// ========== Инициализация ==========
async function init() {
  const me = await guarded(() => Api.me());
  state.me = me;

  if (!me.registered) return showScreen('welcome');
  if (!me.quizDone) {
    await loadQuizQuestions();
    return showScreen('quiz');
  }
  showScreen('home');
}

// ========== 1. Приветственный экран ==========
const nameInput = document.getElementById('user-name');
const ageGroup = document.getElementById('age-group');
const photoInput = document.getElementById('photo-input');
const photoPreview = document.getElementById('photo-preview');
const btnContinue = document.getElementById('btn-welcome-continue');

function checkWelcome() {
  const name = nameInput.value.trim();
  const age = document.querySelector('input[name="age"]:checked');
  btnContinue.disabled = !(name && age && state.photos.length >= 1);
}

nameInput.addEventListener('input', checkWelcome);
ageGroup.addEventListener('change', e => {
  document.querySelectorAll('#age-group .option').forEach(o => o.classList.remove('selected'));
  if (e.target.checked) e.target.closest('.option').classList.add('selected');
  checkWelcome();
});

photoInput.addEventListener('change', () => {
  const files = Array.from(photoInput.files).slice(0, 3);
  state.photos = [];
  photoPreview.innerHTML = '';
  files.forEach(file => {
    const url = URL.createObjectURL(file);
    state.photos.push(url);
    const img = document.createElement('img');
    img.src = url;
    img.className = 'photo-thumb';
    photoPreview.appendChild(img);
  });
  if (state.photos.length < 3) {
    const plus = document.createElement('div');
    plus.className = 'photo-upload';
    plus.textContent = '+';
    plus.onclick = () => photoInput.click();
    photoPreview.appendChild(plus);
  }
  checkWelcome();
});

btnContinue.addEventListener('click', () => {
  btnContinue.classList.add('loading');
  guarded(async () => {
    const refCode = tg?.initDataUnsafe?.start_param || null;
    await Api.onboarding({
      name: nameInput.value.trim(),
      ageGroup: document.querySelector('input[name="age"]:checked').value,
      photos: state.photos.map((_, i) => `local-photo-${i}`),
      refCode
    });
    await loadQuizQuestions();
    showScreen('quiz');
  }).finally(() => btnContinue.classList.remove('loading'));
});

// ========== 2. Анкета ==========
async function loadQuizQuestions() {
  if (state.quizQuestions.length) return;
  const { questions } = await Api.quizQuestions();
  state.quizQuestions = questions;
}

function renderQuestion() {
  const q = state.quizQuestions[state.quizIndex];
  const total = state.quizQuestions.length;
  document.getElementById('quiz-progress-text').textContent = `Вопрос ${state.quizIndex + 1} из ${total}`;
  document.getElementById('quiz-step').textContent = `${state.quizIndex + 1} / ${total}`;
  document.getElementById('quiz-bar').style.width = `${((state.quizIndex + 1) / total) * 100}%`;

  let html = q.cat ? `<div style="font-size:12px;color:var(--tg-hint-color);margin-bottom:6px">${q.cat}</div>` : '';
  html += `<h2 style="margin-bottom:16px">${q.text}</h2>`;

  if (q.type === 'single' || q.type === 'multi') {
    html += '<div class="options">';
    q.options.forEach(opt => {
      const value = typeof opt === 'object' ? opt.value : opt;
      const label = typeof opt === 'object' ? opt.label : opt;
      const current = state.quizAnswers[q.id];
      const checked = Array.isArray(current) ? current.includes(value) : current === value;
      const type = q.type === 'single' ? 'radio' : 'checkbox';
      html += `<label class="option ${checked ? 'selected' : ''}">
        <input type="${type}" name="q${q.id}" value="${value}" ${checked ? 'checked' : ''}>
        ${label}
      </label>`;
    });
    html += '</div>';
  } else if (q.type === 'scale') {
    html += '<div class="options" style="flex-direction:row;flex-wrap:wrap;gap:8px">';
    for (let i = 1; i <= 10; i++) {
      const selected = state.quizAnswers[q.id] == i;
      html += `<div class="option ${selected ? 'selected' : ''}" style="width:42px;justify-content:center;padding:10px 0" data-val="${i}">${i}</div>`;
    }
    html += '</div>';
  } else if (q.type === 'text') {
    html += `<input type="text" id="q-text" value="${state.quizAnswers[q.id] || ''}" placeholder="Твой ответ...">`;
  }

  document.getElementById('quiz-question').innerHTML = html;

  document.querySelectorAll('#quiz-question .option').forEach(opt => {
    opt.addEventListener('click', () => {
      const input = opt.querySelector('input');
      if (input) {
        if (input.type === 'radio') {
          document.querySelectorAll(`input[name="${input.name}"]`).forEach(i => i.closest('.option').classList.remove('selected'));
          opt.classList.add('selected');
          input.checked = true;
          state.quizAnswers[q.id] = input.value;
        } else {
          input.checked = !input.checked;
          opt.classList.toggle('selected', input.checked);
          const vals = Array.from(document.querySelectorAll(`input[name="${input.name}"]:checked`)).map(i => i.value);
          state.quizAnswers[q.id] = vals;
        }
      } else {
        document.querySelectorAll('#quiz-question .option').forEach(o => o.classList.remove('selected'));
        opt.classList.add('selected');
        state.quizAnswers[q.id] = opt.dataset.val;
      }
    });
  });

  document.getElementById('btn-quiz-back').style.visibility = state.quizIndex === 0 ? 'hidden' : 'visible';
  document.getElementById('btn-quiz-next').textContent = state.quizIndex === total - 1 ? 'Готово' : 'Далее';
}

document.getElementById('btn-quiz-next').addEventListener('click', () => {
  const q = state.quizQuestions[state.quizIndex];
  if (q.type === 'text') {
    const val = document.getElementById('q-text')?.value.trim();
    if (!val) return showToast('Напиши ответ');
    state.quizAnswers[q.id] = val;
  } else if (!state.quizAnswers[q.id] || (Array.isArray(state.quizAnswers[q.id]) && !state.quizAnswers[q.id].length)) {
    return showToast('Выбери вариант');
  }

  if (state.quizIndex < state.quizQuestions.length - 1) {
    state.quizIndex++;
    renderQuestion();
  } else {
    guarded(async () => {
      const result = await Api.submitQuiz(state.quizAnswers);
      showToast(`Анкета сохранена! +${result.xpGain} XP`);
      setTimeout(() => {
        const areas = (result.focusAreas || []).map(c => `${CATEGORY_ICON[c] || ''} ${c}`).join('\n');
        celebrate('🎯', 'Твой план готов', areas
          ? `По ответам мы поняли, что сейчас тебе больше всего важно подтянуть:\n\n${areas}\n\nПлан на 365 дней построен с упором на это.`
          : 'План на 365 дней готов — двигаемся вместе.');
        showScreen('home');
      }, 500);
    });
  }
});

document.getElementById('btn-quiz-back').addEventListener('click', () => {
  if (state.quizIndex > 0) {
    state.quizIndex--;
    renderQuestion();
  }
});

// ========== 3. Главный экран ==========
async function loadHome() {
  try {
    const [today, progress] = await Promise.all([Api.today(), Api.progress()]);
    state.today = today;

    const circumference = 2 * Math.PI * 52;
    const offset = circumference - (progress.progressPct / 100) * circumference;
    document.getElementById('home-circle').style.strokeDashoffset = offset;
    document.getElementById('home-pct').textContent = Math.round(progress.progressPct) + '%';
    document.getElementById('home-days').textContent = today.finished
      ? 'Путь пройден полностью'
      : `День ${progress.completedCount + 1} из ${progress.totalTasks}`;

    document.getElementById('today-category').textContent = `${CATEGORY_ICON[today.category] || ''} ${today.category}`;
    document.getElementById('today-action').textContent = today.text;

    const buttons = document.getElementById('action-buttons');
    const doneState = document.getElementById('action-done-state');
    const finishedState = document.getElementById('action-finished-state');

    if (today.finished) {
      buttons.style.display = 'none';
      doneState.style.display = 'none';
      finishedState.style.display = 'block';
    } else if (!today.canActToday) {
      buttons.style.display = 'none';
      doneState.style.display = 'block';
      finishedState.style.display = 'none';
    } else {
      buttons.style.display = 'block';
      doneState.style.display = 'none';
      finishedState.style.display = 'none';
    }
  } catch (err) {
    if (err.status === 409 && err.data && err.data.quizDone === false) {
      await loadQuizQuestions();
      showScreen('quiz');
    }
  }
}

document.getElementById('btn-done').addEventListener('click', () => showScreen('diary'));
document.getElementById('btn-skip').addEventListener('click', () => openModal('modal-skip'));

document.getElementById('confirm-skip').addEventListener('click', () => {
  guarded(async () => {
    closeModal('modal-skip');
    await Api.skipToday();
    showToast('Действие пропущено — увидимся завтра');
    await loadHome();
  });
});

document.getElementById('btn-why').addEventListener('click', () => {
  document.getElementById('why-text').textContent = state.today?.why || '';
  openModal('modal-why');
});

// ========== 4. Прогресс ==========
async function loadProgress() {
  const p = await guarded(() => Api.progress());
  document.getElementById('stat-level').textContent = p.level;
  document.getElementById('stat-xp').textContent = p.xp;
  document.getElementById('stat-tasks').textContent = `${p.completedCount} / ${p.totalTasks}`;
  document.getElementById('stat-streak').textContent = `${p.streakCurrent} дней ${p.streakCurrent > 0 ? '🔥' : ''}`;

  document.getElementById('categories-list').innerHTML = p.categories.map(c => `
    <div class="category-item">
      <div class="category-name">${CATEGORY_ICON[c.name] || ''} ${c.name}</div>
      <div class="category-bar">
        <div class="progress-bar"><div class="progress-fill" style="width:${c.pct}%"></div></div>
      </div>
      <div class="category-pct">${c.pct}%</div>
    </div>
  `).join('');
}

// ========== 5. Дневник ==========
document.querySelectorAll('.emoji-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.emoji-btn').forEach(b => b.classList.remove('selected'));
    btn.classList.add('selected');
    state.selectedEmoji = btn.dataset.val;
  });
});

document.getElementById('btn-save-diary').addEventListener('click', () => {
  if (!state.selectedEmoji) return showToast('Выбери эмоцию');
  guarded(async () => {
    const note = document.getElementById('diary-note').value;
    const result = await Api.saveDiary(state.selectedEmoji, note);

    showToast(`Задание выполнено! +${result.xpGain} XP`);
    document.getElementById('diary-note').value = '';
    document.querySelectorAll('.emoji-btn').forEach(b => b.classList.remove('selected'));
    state.selectedEmoji = null;

    setTimeout(async () => {
      if (result.reachedLevel100) {
        celebrate('💎', 'УРОВЕНЬ 100!', `Ты полностью собрал(а) свою Version 2.0 за 365 дней! В подарок — ${result.premiumGrantedMonths} месяцев Premium-подписки. Твой алмаз теперь виден всем в «Сообществе».`);
      } else if (result.unlocked && result.unlocked.length) {
        const a = result.unlocked[0];
        celebrate(a.icon, a.name, a.desc);
      }
      await showScreen('home');
    }, 300);
  });
});

async function loadDiaryExtras() {
  try {
    const report = await Api.monthlyReport();
    const el = document.getElementById('mood-trend');
    if (!report.moodTrend.length) {
      el.innerHTML = '<div class="subtitle" style="margin:0">Пока недостаточно записей — заполняй дневник каждый день</div>';
    } else {
      el.innerHTML = report.moodTrend.map(m => `<span class="mood-chip">Нед. ${m.week}: ${m.avgMood}/5</span>`).join('');
    }
  } catch { /* необязательный блок */ }
}

// ========== 6. Сообщество ==========
let communityTab = 'all';

document.querySelectorAll('#community-tabs button').forEach(btn => {
  btn.addEventListener('click', () => {
    if (btn.id === 'tab-friends' && btn.disabled) return;
    document.querySelectorAll('#community-tabs button').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    communityTab = btn.dataset.tab;
    loadCommunity(communityTab);
  });
});

function renderPersonRow(u) {
  const diamond = u.diamond ? '<span class="diamond-badge">💎</span>' : '';
  const badges = (u.badges || []).filter(c => c !== 'lvl100')
    .map(c => ACHIEVEMENT_ICONS[c] || '').join(' ');
  const levelChip = u.level !== undefined ? `<div class="list-level">Ур. ${u.level}</div>` : '';
  return `
    <div class="list-item">
      <div class="list-avatar">${u.gender === 'female' ? '👩' : u.gender === 'male' ? '🧑' : '👤'}${diamond}</div>
      <div style="flex:1">
        <div class="list-name">${u.name || 'Без имени'} <span class="badge-mini">${badges}</span></div>
      </div>
      ${levelChip}
    </div>`;
}

const ACHIEVEMENT_ICONS = { '7d': '🔥', '30d': '💪', '180d': '🚀', '365d': '👑', 'lvl25': '⭐', 'lvl50': '🌟', 'lvl75': '✨' };

async function loadCommunity(tab) {
  const upsell = document.getElementById('community-upsell');
  const listEl = document.getElementById('community-list');
  const friendsTabBtn = document.getElementById('tab-friends');
  const addFriendBlock = document.getElementById('add-friend-block');

  if (tab === 'friends') {
    addFriendBlock.style.display = 'block';
    try {
      const { friends } = await Api.friends();
      upsell.style.display = 'none';
      listEl.innerHTML = friends.length
        ? friends.map(renderPersonRow).join('')
        : '<div class="subtitle" style="margin:0">Пока никого нет — пригласи друга или найди его по @username</div>';
    } catch (err) {
      listEl.innerHTML = '';
      upsell.style.display = 'block';
      upsell.textContent = err.message;
    }
    return;
  }

  addFriendBlock.style.display = 'none';
  const data = await guarded(() => Api.community());
  friendsTabBtn.disabled = !data.premiumView;
  friendsTabBtn.style.opacity = data.premiumView ? '1' : '0.5';

  if (!data.premiumView) {
    upsell.style.display = 'block';
    upsell.textContent = data.upsell;
  } else {
    upsell.style.display = 'none';
  }

  listEl.innerHTML = data.users.map(renderPersonRow).join('');
}

document.getElementById('btn-add-friend').addEventListener('click', () => {
  const input = document.getElementById('friend-username');
  const username = input.value.trim();
  if (!username) return showToast('Введи @username');
  guarded(async () => {
    const res = await Api.addFriend(username);
    showToast(`${res.friend.name} добавлен(а) в контакты`);
    input.value = '';
    await loadCommunity('friends');
  });
});

// ========== 7. Достижения ==========
async function loadAchievements() {
  const { achievements } = await guarded(() => Api.achievements());
  document.getElementById('achievements-grid').innerHTML = achievements.map(a => `
    <div class="achievement ${a.unlocked ? 'unlocked' : ''}">
      <div class="icon">${a.icon}</div>
      <div class="name">${a.name}</div>
      <div class="desc">${a.desc}</div>
    </div>
  `).join('');
}

// ========== 8. Профиль ==========
async function loadProfile() {
  const [me, quote, progress] = await Promise.all([
    guarded(() => Api.me()), guarded(() => Api.subscribeQuote()), guarded(() => Api.progress())
  ]);
  state.me = me;

  document.getElementById('profile-avatar').textContent = me.gender === 'female' ? '👩' : me.gender === 'male' ? '🧑' : '👤';
  document.getElementById('profile-name').textContent = me.name || 'Без имени';
  document.getElementById('profile-age').textContent = me.ageGroup || '';

  const tierLabel = { free: 'Бесплатно', plus369: 'Version 2.0 Plus', premium888: 'Version 2.0 Premium 💎' };
  document.getElementById('sub-status').textContent = tierLabel[me.subscriptionTier] || 'Бесплатно';

  document.getElementById('p-progress').textContent = Math.round(me.progressPct) + '%';
  document.getElementById('p-level').textContent = me.level;
  document.getElementById('p-xp').textContent = me.xp;
  document.getElementById('p-streak').textContent = progress.streakCurrent;

  document.getElementById('ref-code').textContent = me.referralCode || '—';

  for (const tier of ['plus369', 'premium888']) {
    const q = quote.quote[tier];
    const priceEl = document.getElementById(`price-${tier}`);
    if (q.discountPct > 0) {
      priceEl.innerHTML = `<span class="old-price">${q.basePrice} ₽</span>${q.pricePaid} ₽/мес <span style="font-size:12px;color:var(--tg-button-color)">(-${q.discountPct}%)</span>`;
    } else {
      priceEl.textContent = `${q.pricePaid} ₽/мес`;
    }
  }

  document.getElementById('btn-wallet').style.display = me.hasPremiumAgentCode ? 'flex' : 'none';
}

document.querySelectorAll('[data-tier]').forEach(btn => {
  btn.addEventListener('click', () => {
    guarded(async () => {
      const res = await Api.subscribe(btn.dataset.tier);
      showToast(`Подписка оформлена: ${res.pricePaid} ₽/мес`);
      await loadProfile();
    });
  });
});

document.getElementById('btn-copy-ref').addEventListener('click', () => {
  const code = document.getElementById('ref-code').textContent;
  navigator.clipboard?.writeText(code).then(() => showToast('Код скопирован'));
});

document.getElementById('btn-share-ref').addEventListener('click', () => {
  const code = state.me?.referralCode;
  if (!code) return;
  const link = BOT_USERNAME
    ? `https://t.me/${BOT_USERNAME}?start=${code}`
    : code;
  if (tg?.openTelegramLink && BOT_USERNAME) {
    tg.openTelegramLink(`https://t.me/share/url?url=${encodeURIComponent(link)}&text=${encodeURIComponent('Присоединяйся к Version 2.0 — стань лучшей версией себя за 365 дней!')}`);
  } else {
    navigator.clipboard?.writeText(link);
    showToast('Ссылка скопирована — перешли её другу');
  }
});

document.getElementById('btn-logout').addEventListener('click', () => {
  if (confirm('Это очистит локальный кэш на этом устройстве (не удаляет твой прогресс на сервере). Продолжить?')) {
    localStorage.removeItem('v2_debug_tg_id');
    location.reload();
  }
});

document.getElementById('btn-report').addEventListener('click', () => showScreen('report'));
document.getElementById('btn-wallet').addEventListener('click', () => showScreen('wallet'));

document.getElementById('btn-share').addEventListener('click', () => {
  if (tg) {
    tg.showAlert('Карточка отчёта будет отправлена (заглушка)');
  } else {
    showToast('Поделиться (заглушка)');
  }
});

// ========== 9. Отчёт ==========
async function loadReport() {
  const r = await guarded(() => Api.weeklyReport());
  document.getElementById('report-body').innerHTML = `
    <div class="flex justify-between mb-8"><span>Выполнено заданий</span><strong>${r.completed} / 7</strong></div>
    <div class="flex justify-between mb-8"><span>Активных дней</span><strong>${r.activeDays}</strong></div>
    <div class="flex justify-between mb-8"><span>STREAK</span><strong>${r.streakCurrent} дней</strong></div>
    <div class="flex justify-between mb-8"><span>Главный результат</span><strong>${r.topCategory || '—'}</strong></div>
    <div class="flex justify-between"><span>Фокус следующей недели</span><strong>${r.focusNextWeek || '—'}</strong></div>
  `;
}

// ========== 10. Кошелёк ==========
async function loadWallet() {
  const w = await guarded(() => Api.wallet());
  document.getElementById('wallet-balance').textContent = `${w.balance} ₽`;
  document.getElementById('wallet-count').textContent = w.payingReferrals;
  document.getElementById('wallet-history').innerHTML = w.history.length
    ? w.history.map(h => `
        <div class="flex justify-between mb-8">
          <span class="subtitle" style="margin:0">${new Date(h.created_at).toLocaleDateString('ru-RU')} · ${h.note || ''}</span>
          <strong>+${h.amount} ₽</strong>
        </div>`).join('')
    : '<div class="subtitle" style="margin:0">Пока пусто</div>';
}

document.getElementById('btn-wallet-withdraw').addEventListener('click', () => {
  guarded(async () => {
    const res = await Api.walletWithdraw();
    showToast(res.message);
  });
});

// ========== Нижняя навигация ==========
document.querySelectorAll('.nav-item').forEach(item => {
  item.addEventListener('click', () => showScreen(item.dataset.screen));
});

init();
