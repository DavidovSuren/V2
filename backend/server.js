require('dotenv').config();

const path = require('path');
const express = require('express');
const cors = require('cors');

require('./db'); // инициализирует схему при первом импорте

const onboardingRoutes = require('./routes/onboarding');
const taskRoutes = require('./routes/tasks');
const diaryRoutes = require('./routes/diary');
const progressRoutes = require('./routes/progress');
const communityRoutes = require('./routes/community');
const subscriptionRoutes = require('./routes/subscriptions');
const adminRoutes = require('./routes/admin');
const reportRoutes = require('./routes/reports');

const app = express();
app.use(cors());
app.use(express.json({ limit: '2mb' }));

app.get('/api/health', (req, res) => res.json({ ok: true }));

// Раздача фронтенда с того же сервера — удобно для локального теста через
// туннель (один HTTPS-адрес и для Mini App, и для API). В проде фронтенд
// можно так же оставить тут или захостить отдельно (GitHub Pages и т.п.).
app.use(express.static(path.join(__dirname, '..', 'frontend')));

app.use('/api', onboardingRoutes);
app.use('/api', taskRoutes);
app.use('/api', diaryRoutes);
app.use('/api', progressRoutes);
app.use('/api', communityRoutes);
app.use('/api', subscriptionRoutes);
app.use('/api', adminRoutes);
app.use('/api', reportRoutes);

app.use((err, req, res, next) => {
  console.error(err);
  res.status(500).json({ error: 'Внутренняя ошибка сервера' });
});

const PORT = process.env.PORT || 3000;

if (require.main === module) {
  app.listen(PORT, () => {
    console.log(`Version 2.0 backend слушает порт ${PORT}`);
    if (!process.env.BOT_TOKEN) {
      console.warn('[server] BOT_TOKEN не задан — уведомления и сообщения бота отключены.');
    }
    if (!process.env.ADMIN_TG_ID) {
      console.warn('[server] ADMIN_TG_ID не задан — выдача премиум-агентских кодов недоступна.');
    }

    require('./cron/dailyReminder').start();
    require('./cron/weeklyReport').start();
    require('./cron/monthlyReport').start();
  });
}

module.exports = app;
