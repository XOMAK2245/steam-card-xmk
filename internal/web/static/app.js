const $ = id => document.getElementById(id);
const T = {
  en:{quit:'Quit',loginTitle:'Sign in to Steam',qrHelp:'Scan the QR code with the Steam mobile app (Steam Guard → “Sign in with QR code”). No password is entered here.',qrBtn:'Show QR code',ckHelp:'Fallback: paste cookies from steamcommunity.com (browser → DevTools → Storage → Cookies).',ckBtn:'Import cookies',start:'▶ Start',pause:'⏸ Pause',stop:'⏹ Stop',refresh:'⟳ Refresh list',strategy:'Strategy',smart:'Smart (parallel, then one by one)',sequential:'Sequential (one game at a time)',parallel:'Max parallel',thr:'Parallel until (h)',poll:'Check every (min)',save:'Save',logout:'Log out',games:'Games',name:'Game',left:'Cards left',hours:'Hours',state:'State',log:'Log',
    steamOn:'Steam: running',steamOff:'Steam: not running',authOn:'Account: ',authOff:'Not signed in',running:'Farming',paused:'Paused',stopped:'Stopped',queued:'queued',active:'farming',done:'done',failed:'failed',qrWait:'Waiting for scan…',qrScanned:'Scanned — confirm on your phone…',qrDone:'Signed in ✓',gamesInfo:(n,c)=>`${n} with drops · ${c} cards left`,nextPoll:'next check'},
  ru:{quit:'Выход',loginTitle:'Вход в Steam',qrHelp:'Отсканируйте QR-код в мобильном приложении Steam (Steam Guard → «Войти по QR-коду»). Пароль здесь не вводится.',qrBtn:'Показать QR-код',ckHelp:'Запасной вариант: вставьте cookies с steamcommunity.com (браузер → DevTools → Storage → Cookies).',ckBtn:'Импорт cookies',start:'▶ Старт',pause:'⏸ Пауза',stop:'⏹ Стоп',refresh:'⟳ Обновить список',strategy:'Стратегия',smart:'Умная (параллельно, затем по одной)',sequential:'Последовательная (по одной игре)',parallel:'Макс. параллельно',thr:'Параллельно до (ч)',poll:'Проверка каждые (мин)',save:'Сохранить',logout:'Выйти',games:'Игры',name:'Игра',left:'Осталось карт',hours:'Часы',state:'Статус',log:'Лог',
    steamOn:'Steam: запущен',steamOff:'Steam: не запущен',authOn:'Аккаунт: ',authOff:'Не авторизован',running:'Фарм идёт',paused:'Пауза',stopped:'Остановлено',queued:'в очереди',active:'фармится',done:'готово',failed:'ошибка',qrWait:'Ждём сканирования…',qrScanned:'Отсканировано — подтвердите на телефоне…',qrDone:'Вход выполнен ✓',gamesInfo:(n,c)=>`${n} с дропом · осталось карт: ${c}`,nextPoll:'след. проверка'}
};
const L = T[(navigator.language||'en').startsWith('ru')?'ru':'en'];
document.documentElement.lang = (navigator.language||'en').slice(0,2);
document.querySelectorAll('[data-i18n]').forEach(e => { if (L[e.dataset.i18n]) e.textContent = L[e.dataset.i18n]; });

const api = (path, body) => fetch('/api/'+path, body===undefined ? {} : {method:'POST', headers:{'X-XMK':'1','Content-Type':'application/json'}, body: JSON.stringify(body)}).then(async r => { if(!r.ok) throw new Error(await r.text()); return r.json(); });
const post = (p,b={}) => api(p,b);

let formInit = false, S = null;
const esc = s => String(s).replace(/[&<>"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));

function render(st) {
  S = st;
  const pill = (el, cls, txt) => { $(el).className = 'pill ' + cls; $(el).textContent = txt; };
  pill('pSteam', st.steamRunning ? 'ok' : 'bad', st.steamRunning ? L.steamOn : L.steamOff);
  pill('pAuth', st.loggedIn ? 'ok' : 'warn', st.loggedIn ? L.authOn + st.steamId : L.authOff);
  pill('pStatus', st.status === 'running' ? 'ok' : '', L[st.status] + (st.status==='running' && st.active.length ? ` · ${st.active.length}` : ''));
  const note = st.workerError ? 'Worker files: ' + st.workerError : st.note;
  $('note').hidden = !note; $('note').textContent = note || '';
  $('loginBox').hidden = st.loggedIn;
  $('btnStart').disabled = !st.loggedIn || st.status === 'running';
  $('btnPause').disabled = st.status !== 'running';
  $('btnStop').disabled = st.status === 'stopped';
  $('btnLogout').hidden = !st.loggedIn;
  if (!formInit) {
    formInit = true;
    $('strategy').value = st.settings.strategy; $('maxParallel').value = st.settings.maxParallel;
    $('parHours').value = st.settings.parallelHours; $('pollMin').value = st.settings.pollMinutes;
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

$('btnStart').onclick = () => post('start');
$('btnPause').onclick = () => post('pause');
$('btnStop').onclick = () => post('stop');
$('btnRefresh').onclick = () => post('refresh');
$('btnLogout').onclick = () => post('logout');
$('btnQuit').onclick = () => { post('quit').finally(() => document.body.innerHTML = '<p style="padding:2em">Stopped. You can close this tab.</p>'); };
$('btnSave').onclick = () => post('settings', {strategy:$('strategy').value, maxParallel:+$('maxParallel').value, parallelHours:+$('parHours').value, pollMinutes:+$('pollMin').value});
$('btnCk').onclick = () => { $('ckErr').textContent=''; post('login/cookies', {LoginSecure:$('ckLogin').value, SessionID:$('ckSess').value}).catch(e => $('ckErr').textContent = e.message); };

let qrTimer = null;
$('btnQr').onclick = async () => {
  try { await post('login/qr'); } catch(e) { $('qrWrap').hidden = false; $('qrStatus').textContent = e.message; return; }
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

api('logs').then(ls => { $('log').textContent = ls.join('\n') + (ls.length ? '\n' : ''); $('log').scrollTop = 1e9; }).then(() => {
  const es = new EventSource('/api/events');
  es.addEventListener('state', e => render(JSON.parse(e.data)));
  es.addEventListener('log', e => addLog(e.data));
});
setInterval(() => { if (S && S.steamRunning !== undefined) api('state').then(render).catch(()=>{}); }, 15000);
