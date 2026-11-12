const $ = id => document.getElementById(id);
const T = {
  en:{signin:'Sign in to Steam',saved:'Saved',refreshing:'Refreshing list…',started:'Farming started',paused_:'Paused',stopped_:'Stopped',needLogin:'Sign in to Steam first',cookiesOk:'Cookies imported, loading games…',loggedOut:'Logged out',lost:'Connection to the app lost',quit:'Quit',loginTitle:'Sign in to Steam',qrHelp:'Scan the QR code with the Steam mobile app (Steam Guard → “Sign in with QR code”). No password is entered here.',qrBtn:'Show QR code',ckHelp:'Fallback: paste cookies from steamcommunity.com (browser → DevTools → Storage → Cookies).',ckBtn:'Import cookies',start:'Start',pause:'Pause',stop:'Stop',refresh:'Refresh',strategy:'Strategy',batch:'Batch (up to 32 games at once)',single:'Sequential (one game at a time)',parallel:'Max parallel',poll:'Check every (min)',save:'Save',logout:'Log out',games:'Games',name:'Game',left:'Cards left',hours:'Hours',state:'State',log:'Log',
    steamOn:'Steam: running',steamOff:'Steam: not running',authOn:'Account: ',authOff:'Not signed in',running:'Farming',paused:'Paused',stopped:'Stopped',queued:'Queued',active:'Farming',done:'Done',failed:'Failed',qrWait:'Waiting for scan…',qrScanned:'Scanned — confirm on your phone…',qrDone:'Signed in',gamesInfo:(n,c)=>`${n} with drops · ${c} cards left`,nextPoll:'next check'},
  ru:{signin:'Войти в Steam',saved:'Сохранено',refreshing:'Обновление списка…',started:'Фарм запущен',paused_:'Поставлено на паузу',stopped_:'Остановлено',needLogin:'Сначала войдите в Steam',cookiesOk:'Cookies импортированы, загружаем игры…',loggedOut:'Вы вышли из аккаунта',lost:'Связь с приложением потеряна',quit:'Выход',loginTitle:'Вход в Steam',qrHelp:'Отсканируйте QR-код в мобильном приложении Steam (Steam Guard → «Войти по QR-коду»). Пароль здесь не вводится.',qrBtn:'Показать QR-код',ckHelp:'Запасной вариант: вставьте cookies с steamcommunity.com (браузер → DevTools → Storage → Cookies).',ckBtn:'Импорт cookies',start:'Старт',pause:'Пауза',stop:'Стоп',refresh:'Обновить список',strategy:'Стратегия',batch:'Параллельный (до 32 игр одновременно)',single:'Последовательный (по одной игре)',parallel:'Макс. параллельно',poll:'Проверка каждые (мин)',save:'Сохранить',logout:'Выйти',games:'Игры',name:'Игра',left:'Осталось карт',hours:'Часы',state:'Статус',log:'Лог',
    steamOn:'Steam: запущен',steamOff:'Steam: не запущен',authOn:'Аккаунт: ',authOff:'Не авторизован',running:'Фарм идёт',paused:'Пауза',stopped:'Остановлено',queued:'В очереди',active:'Фармится',done:'Готово',failed:'Ошибка',qrWait:'Ждём сканирования…',qrScanned:'Отсканировано — подтвердите на телефоне…',qrDone:'Вход выполнен',gamesInfo:(n,c)=>`${n} с дропом · осталось карт: ${c}`,nextPoll:'след. проверка'}
};
const L = T[(navigator.language||'en').startsWith('ru')?'ru':'en'];
document.documentElement.lang = (navigator.language||'en').slice(0,2);
document.querySelectorAll('[data-i18n]').forEach(e => { if (L[e.dataset.i18n]) e.textContent = L[e.dataset.i18n]; });

let toastTimer = null;
function toast(msg, kind) {
  const el = $('toast');
  el.textContent = msg; el.className = 'toast ' + (kind || ''); el.hidden = false;
  clearTimeout(toastTimer); toastTimer = setTimeout(() => el.hidden = true, 3500);
}
async function api(path, body) {
  const opt = body === undefined ? {} : {method:'POST', headers:{'X-XMK':'1','Content-Type':'application/json'}, body: JSON.stringify(body)};
  const r = await fetch('/api/' + path, opt);
  if (!r.ok) throw new Error((await r.text()).trim() || ('HTTP ' + r.status));
  return r.json();
}
const post = (p, b = {}) => api(p, b);
async function act(fn, okMsg) {
  try { const r = await fn(); if (okMsg) toast(okMsg, 'ok'); return r; }
  catch (e) { toast(e.message, 'err'); }
}

let formInit = false, S = null, showLogin = false;
const esc = s => String(s).replace(/[&<>"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));

function render(st) {
  S = st;
  const pill = (el, cls, txt) => { $(el).className = 'pill ' + cls; $(el).textContent = txt; };
  pill('pSteam', st.steamRunning ? 'ok' : 'bad', st.steamRunning ? L.steamOn : L.steamOff);
  pill('pAuth', st.loggedIn ? 'ok' : 'warn', st.loggedIn ? L.authOn + st.steamId : L.authOff);
  pill('pStatus', st.status === 'running' ? 'ok' : '', L[st.status] + (st.status==='running' && st.active.length ? ` · ${st.active.length}` : ''));
  const note = st.workerError ? 'Worker files: ' + st.workerError : st.note;
  $('note').hidden = !note; $('note').textContent = note || '';
  $('loginBox').hidden = st.loggedIn && !showLogin;
  $('btnLoginShow').hidden = st.loggedIn;
  $('btnLogout').hidden = !st.loggedIn;
  if (!formInit) {
    formInit = true;
    $('strategy').value = st.settings.strategy || 'batch';
    $('maxParallel').value = st.settings.maxParallel || 32;
    $('pollMin').value = st.settings.pollMinutes || 5;
  }
  const rank = {active:0, queued:1, failed:2, done:3};
  const games = [...st.games].sort((a,b) => rank[a.state]-rank[b.state] || b.hours-a.hours);
  $('games').innerHTML = games.map(g => `<tr class="${g.state}"><td>${g.appId}</td><td>${esc(g.name||'?')}</td><td>${g.remaining}</td><td>${g.hours.toFixed(1)}</td><td><span class="tag ${g.state}">${L[g.state]}</span></td></tr>`).join('');
  const pending = st.games.filter(g => g.state !== 'done');
  let info = L.gamesInfo(pending.length, pending.reduce((a,g)=>a+g.remaining,0));
  if (st.nextPoll) info += ` · ${L.nextPoll} ${new Date(st.nextPoll).toLocaleTimeString()}`;
  $('gamesInfo').textContent = info;
}

function addLog(line) {
  const el = $('log'), atEnd = el.scrollTop + el.clientHeight >= el.scrollHeight - 20;
  el.textContent += line + '\n';
  if (atEnd) el.scrollTop = el.scrollHeight;
}

$('btnStart').onclick = () => {
  if (S && !S.loggedIn) { showLogin = true; render(S); return toast(L.needLogin, 'err'); }
  act(() => post('start'), L.started);
};
$('btnPause').onclick = () => act(() => post('pause'), L.paused_);
$('btnStop').onclick = () => act(() => post('stop'), L.stopped_);
$('btnRefresh').onclick = () => {
  if (S && !S.loggedIn) { showLogin = true; render(S); return toast(L.needLogin, 'err'); }
  act(() => post('refresh'), L.refreshing);
};
$('btnLogout').onclick = () => act(() => post('logout'), L.loggedOut);
$('btnLoginShow').onclick = () => { showLogin = !showLogin; if (S) render(S); };
$('btnQuit').onclick = async () => {
  try { await post('quit'); } catch (e) {}
  document.body.innerHTML = '<p style="padding:2em">Stopped. You can close this window.</p>';
  window.close();
};
$('btnSave').onclick = () => act(() => post('settings', {strategy:$('strategy').value, maxParallel:+$('maxParallel').value, pollMinutes:+$('pollMin').value}).then(render), L.saved);
$('btnCk').onclick = async () => {
  $('ckErr').textContent = '';
  try { await post('login/cookies', {LoginSecure:$('ckLogin').value, SessionID:$('ckSess').value}); toast(L.cookiesOk, 'ok'); }
  catch (e) { $('ckErr').textContent = e.message; toast(e.message, 'err'); }
};

let qrTimer = null;
$('btnQr').onclick = async () => {
  try { await post('login/qr'); } catch(e) { $('qrWrap').hidden = false; $('qrStatus').textContent = e.message; toast(e.message, 'err'); return; }
  $('qrWrap').hidden = false;
  clearInterval(qrTimer);
  let last = '';
  qrTimer = setInterval(async () => {
    const q = await api('login/qr');
    if (q.url && q.url !== last) { last = q.url; $('qrImg').src = '/api/login/qr.png?' + Date.now(); }
    $('qrStatus').textContent = q.error || ({waiting:L.qrWait, scanned:L.qrScanned, done:L.qrDone}[q.status] || '');
    if (q.status === 'done' || q.status === 'error') { clearInterval(qrTimer); if (q.status==='done') $('qrWrap').hidden = true; }
  }, 1500);
};

// Each startup request is independent: one failing must never break the UI.
api('state').then(render).catch(e => toast(e.message, 'err'));
api('logs').then(ls => {
  ls = Array.isArray(ls) ? ls : [];
  $('log').textContent = ls.join('\n') + (ls.length ? '\n' : '');
  $('log').scrollTop = 1e9;
}).catch(() => {});
try {
  const es = new EventSource('/api/events');
  es.addEventListener('state', e => { try { render(JSON.parse(e.data)); } catch (x) {} });
  es.addEventListener('log', e => addLog(e.data));
  es.onerror = () => { $('pStatus').className = 'pill bad'; $('pStatus').textContent = L.lost; };
} catch (e) { toast(e.message, 'err'); }
setInterval(() => api('state').then(render).catch(() => {}), 10000);
