// Держит SSH-туннель (serveo.net) живым: если бесплатный туннель
// оборвётся (это у него бывает), поднимает новый и сам обновляет кнопку
// меню бота на свежий адрес — без ручного вмешательства.
const path = require('path');
require('dotenv').config({ path: path.join(__dirname, '..', '.env') });
const { spawn } = require('child_process');

const BOT_TOKEN = process.env.BOT_TOKEN;

async function setMenuButton(url) {
  try {
    const res = await fetch(`https://api.telegram.org/bot${BOT_TOKEN}/setChatMenuButton`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        menu_button: { type: 'web_app', text: 'Открыть Version 2.0', web_app: { url } }
      })
    });
    const data = await res.json();
    console.log(`[tunnel-supervisor] кнопка меню обновлена на ${url}:`, data.ok);
  } catch (err) {
    console.error('[tunnel-supervisor] не удалось обновить кнопку меню:', err.message);
  }
}

let restarting = false;

function startTunnel() {
  console.log('[tunnel-supervisor] запускаю ssh-туннель...');
  const proc = spawn('ssh', [
    '-o', 'StrictHostKeyChecking=no',
    '-o', 'ServerAliveInterval=10',
    '-o', 'ServerAliveCountMax=3',
    '-R', '80:localhost:3000',
    'serveo.net'
  ]);

  let buffer = '';
  let lastUrl = null;
  const onOutput = (data) => {
    process.stderr.write('[ssh] ' + data.toString());
    buffer += data.toString();
    const match = buffer.match(/https:\/\/[a-z0-9.-]+\.serveousercontent\.com/);
    if (match && match[0] !== lastUrl) {
      lastUrl = match[0];
      console.log('[tunnel-supervisor] новый адрес:', match[0]);
      setMenuButton(match[0]);
    }
  };

  // serveo.net печатает адрес пересылки в stderr, ssh-диагностику — тоже
  // туда, поэтому слушаем оба потока на всякий случай.
  proc.stdout.on('data', onOutput);
  proc.stderr.on('data', onOutput);

  proc.on('exit', (code) => {
    console.log(`[tunnel-supervisor] ssh завершился (код ${code}), перезапуск через 2с...`);
    setTimeout(startTunnel, 2000);
  });
}

startTunnel();
