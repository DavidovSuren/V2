// Адрес backend API. По умолчанию — тот же origin, с которого открыт
// фронтенд (актуально, когда backend раздаёт frontend/ сам — см. server.js).
// Если фронтенд хостится отдельно от backend, задай window.V2_API_BASE
// абсолютным URL до <script src="api.js"> (см. README.md, раздел "Деплой").
const API_BASE = window.V2_API_BASE || `${window.location.origin}/api`;

const tgApp = window.Telegram?.WebApp;

// В реальном Telegram Mini App аутентификация идёт через initData.
// Вне Telegram (обычный браузер, локальная разработка) используем
// заголовки X-Debug-*, которые backend принимает только при
// DEV_ALLOW_FAKE_AUTH=true — см. backend/.env.example.
function authHeaders() {
  if (tgApp?.initData) {
    return { 'Authorization': `tma ${tgApp.initData}` };
  }
  let debugId = localStorage.getItem('v2_debug_tg_id');
  if (!debugId) {
    debugId = 'local-' + Math.random().toString(36).slice(2, 10);
    localStorage.setItem('v2_debug_tg_id', debugId);
  }
  return {
    'X-Debug-Tg-Id': debugId,
    'X-Debug-Username': 'local_tester',
    // HTTP-заголовки не поддерживают кириллицу напрямую — кодируем.
    'X-Debug-Name': encodeURIComponent('Тестовый пользователь')
  };
}

async function api(path, { method = 'GET', body } = {}) {
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders()
    },
    body: body !== undefined ? JSON.stringify(body) : undefined
  });

  let data = null;
  try { data = await res.json(); } catch { /* no body */ }

  if (!res.ok) {
    const error = new Error(data?.error || `Ошибка запроса (${res.status})`);
    error.status = res.status;
    error.data = data;
    throw error;
  }
  return data;
}

const Api = {
  me: () => api('/me'),
  onboarding: (payload) => api('/onboarding', { method: 'POST', body: payload }),
  quizQuestions: () => api('/quiz/questions'),
  submitQuiz: (answers) => api('/quiz', { method: 'POST', body: { answers } }),

  today: () => api('/today'),
  skipToday: () => api('/today/skip', { method: 'POST' }),
  saveDiary: (emoji, note) => api('/diary', { method: 'POST', body: { emoji, note } }),
  diaryHistory: (limit = 60) => api(`/diary?limit=${limit}`),

  progress: () => api('/progress'),
  achievements: () => api('/achievements'),

  community: () => api('/community'),
  friends: () => api('/friends'),
  addFriend: (username) => api('/friends/add', { method: 'POST', body: { username } }),

  subscribeQuote: () => api('/subscribe/quote'),
  subscribe: (tier) => api('/subscribe', { method: 'POST', body: { tier } }),
  wallet: () => api('/wallet'),
  walletWithdraw: () => api('/wallet/withdraw', { method: 'POST' }),

  weeklyReport: () => api('/reports/weekly'),
  monthlyReport: () => api('/reports/monthly')
};
