const $ = id => document.getElementById(id);
const T = {
  en:{
    subtitle:'Ultimate Gamer Utility',signin:'Sign in to Steam',saved:'Saved',refreshing:'Refreshing list…',started:'Farming started',paused_:'Paused',stopped_:'Stopped',needLogin:'Sign in to Steam first',cookiesOk:'Cookies imported, loading games…',loggedOut:'Logged out',lost:'Connection to the app lost',quit:'Quit',loginTitle:'Sign in to Steam',qrHelp:'Scan the QR code with the Steam mobile app (Steam Guard → “Sign in with QR code”). No password is entered here.',qrBtn:'Show QR code',ckHelp:'Fallback: paste cookies from steamcommunity.com (browser → DevTools → Storage → Cookies).',ckBtn:'Import cookies',start:'Start',pause:'Pause',stop:'Stop',refresh:'Refresh',strategy:'Strategy',batch:'Batch (up to 32 games at once)',single:'Sequential (one game at a time)',parallel:'Max parallel',poll:'Check every (min)',save:'Save',logout:'Log out',games:'Games',name:'Game',left:'Cards left',hours:'Hours',state:'State',log:'Log',
    steamOn:'Steam: running',steamOff:'Steam: not running',authOn:'Account: ',authOff:'Not signed in',running:'Farming',paused:'Paused',stopped:'Stopped',queued:'Queued',active:'Farming',done:'Done',failed:'Failed',qrWait:'Waiting for scan…',qrScanned:'Scanned — confirm on your phone…',qrDone:'Signed in',gamesInfo:(n,c)=>`${n} with drops · ${c} cards left`,nextPoll:'next check',
    addAccount:'+ Add Account',removeAccount:'Remove',switchAccount:'Switch account',confirmRemove:'Are you sure you want to remove this account?',accountSwitched:'Account switched',accountRemoved:'Account removed',steamNotLoggedIn:'Not signed in',discordNotConnected:'Not connected',
    tabSteam:'Steam Cards',tabDiscord:'Discord Quests',dqSafeTitle:'Anti-Detection Guard Active:',dqSafeDesc:'Official Discord Desktop client signature, X-Super-Properties, natural timing jitter (27–33s heartbeats), video watch pacing and realistic action delays are enabled. Complete quests without launching heavy games or streaming.',dqTokenTitle:'Discord Authorization',dqTokenHelp:'Enter your Discord User Token. It is stored locally in config.json and used only for official Discord API requests.',dqSaveToken:'Save & Connect',dqRemoveToken:'Disconnect',dqHowToTitle:'How to get your Discord Token?',dqHowToBtn:'Instructions',dqStep1:'Open Discord in your browser (discord.com/app) or desktop app.',dqStep2:'Press Ctrl+Shift+I (or F12) to open Developer Tools.',dqStep3:'Go to the Network tab and filter requests by "api".',dqStep4:'Click any request, find Request Headers, and copy the "authorization" header value.',dqStartAll:'Complete All Quests',dqStop:'Stop Quests',dqRefresh:'Refresh Quests',dqAutoEnroll:'Auto-accept quests before starting',dqMode:'Mode:',dqModeSafe:'Safe (Sequential)',dqModeFast:'Fast (Parallel)',dqFastWarn:'Fast mode: parallel requests',dqVideo:'Video',dqWatch:'Watch',dqQuests:'Active Quests',dqEmpty:'No active quests found or Discord token not configured.',dqNoResults:'No quests match the search.',dqClaimedSection:'Completed quests',steamSearchPlaceholder:'Search by name or AppID...',dqSearchPlaceholder:'Search quest...',dqAddAccount:'+ Add Account',dqConnected:'Discord: connected',dqDisconnected:'Discord: not connected',dqSaved:'Discord token saved',dqRemoved:'Discord token removed',dqStarted:'Quest automation started',dqStopped:'Quest automation stopped',dqRunning:'Running',dqClaimed:'Claimed',dqCompleted:'Completed',dqAvailable:'Available',dqEnrolled:'Enrolled',dqNotEnrolled:'Not enrolled',dqEnrollBtn:'Accept',dqEnrolledSuccess:'Quest accepted',dqStartQuest:'Start',dqReward:'Reward',dqDuration:'Duration',dqReadyToClaim:'Ready to Claim',min:'min',sec:'sec'
  },
  ru:{
    subtitle:'Ультимативный геймерский комбайн',signin:'Войти в Steam',saved:'Сохранено',refreshing:'Обновление списка…',started:'Фарм запущен',paused_:'Поставлено на паузу',stopped_:'Остановлено',needLogin:'Сначала войдите в Steam',cookiesOk:'Cookies импортированы, загружаем игры…',loggedOut:'Вы вышли из аккаунта',lost:'Связь с приложением потеряна',quit:'Выход',loginTitle:'Вход в Steam',qrHelp:'Отсканируйте QR-код в мобильном приложении Steam (Steam Guard → «Войти по QR-коду»). Пароль здесь не вводится.',qrBtn:'Показать QR-код',ckHelp:'Запасной вариант: вставьте cookies с steamcommunity.com (браузер → DevTools → Storage → Cookies).',ckBtn:'Импорт cookies',start:'Старт',pause:'Пауза',stop:'Стоп',refresh:'Обновить список',strategy:'Стратегия',batch:'Параллельный (до 32 игр одновременно)',single:'Последовательный (по одной игре)',parallel:'Макс. параллельно',poll:'Проверка каждые (мин)',save:'Сохранить',logout:'Выйти',games:'Игры',name:'Игра',left:'Осталось карт',hours:'Часы',state:'Статус',log:'Лог',
    steamOn:'Steam: запущен',steamOff:'Steam: не запущен',authOn:'Аккаунт: ',authOff:'Не авторизован',running:'Фарм идёт',paused:'Пауза',stopped:'Остановлено',queued:'В очереди',active:'Фармится',done:'Готово',failed:'Ошибка',qrWait:'Ждём сканирования…',qrScanned:'Отсканировано — подтвердите на телефоне…',qrDone:'Вход выполнен',gamesInfo:(n,c)=>`${n} с дропом · осталось карт: ${c}`,nextPoll:'след. проверка',
    addAccount:'+ Добавить аккаунт',removeAccount:'Удалить',switchAccount:'Сменить аккаунт',confirmRemove:'Вы уверены, что хотите удалить этот аккаунт?',accountSwitched:'Аккаунт переключён',accountRemoved:'Аккаунт удалён',steamNotLoggedIn:'Не авторизован',discordNotConnected:'Не подключён',
    tabSteam:'Карточки Steam',tabDiscord:'Квесты Discord',dqSafeTitle:'Защита от детекта активна:',dqSafeDesc:'Используются реальные заголовки официального Desktop клиента Discord, X-Super-Properties, случайный джиттер запросов (27–33 сек), просмотр видео с таймингами и естественные задержки. Квесты выполняются без запуска игр и стримов.',dqTokenTitle:'Авторизация в Discord',dqTokenHelp:'Введите пользовательский токен Discord. Он сохраняется локально в config.json и отправляется только на официальные серверы Discord.',dqSaveToken:'Сохранить и подключиться',dqRemoveToken:'Отключить',dqHowToTitle:'Как получить токен Discord?',dqHowToBtn:'Инструкция',dqStep1:'Откройте Discord в браузере (discord.com/app) или десктопном клиенте.',dqStep2:'Нажмите Ctrl+Shift+I (или F12), чтобы открыть инструменты разработчика.',dqStep3:'Перейдите на вкладку Network (Сеть) и укажите фильтр "api".',dqStep4:'Кликните на любой запрос, найдите Request Headers и скопируйте значение "authorization".',dqStartAll:'Выполнить все квесты',dqStop:'Остановить квесты',dqRefresh:'Обновить квесты',dqAutoEnroll:'Автоматически принимать квесты перед выполнением',dqMode:'Режим:',dqModeSafe:'Безопасный (по очереди)',dqModeFast:'Быстрый (параллельно)',dqFastWarn:'Быстрый режим: параллельные запросы',dqVideo:'Видео',dqWatch:'Смотреть',dqQuests:'Доступные квесты',dqEmpty:'Активных квестов не найдено или токен Discord ещё не указан.',dqNoResults:'Квесты по запросу не найдены.',dqClaimedSection:'Завершённые квесты',steamSearchPlaceholder:'Поиск по названию или AppID...',dqSearchPlaceholder:'Поиск квеста...',dqAddAccount:'+ Добавить аккаунт',dqConnected:'Discord: подключён',dqDisconnected:'Discord: не подключён',dqSaved:'Токен Discord сохранён',dqRemoved:'Токен Discord удалён',dqStarted:'Автовыполнение квестов запущено',dqStopped:'Автовыполнение квестов остановлено',dqRunning:'Выполняется',dqClaimed:'Получено',dqCompleted:'Завершён',dqAvailable:'Доступен',dqEnrolled:'Принят',dqNotEnrolled:'Не принят',dqEnrollBtn:'Принять',dqEnrolledSuccess:'Квест успешно принят',dqStartQuest:'Начать',dqReward:'Награда',dqDuration:'Длительность',dqReadyToClaim:'Заберите в Discord',min:'мин',sec:'сек'
  }
};
const L = T[(navigator.language||'en').startsWith('ru')?'ru':'en'];
document.documentElement.lang = (navigator.language||'en').slice(0,2);
function applyI18n() {
  document.querySelectorAll('[data-i18n]').forEach(e => { if (L[e.dataset.i18n]) e.textContent = L[e.dataset.i18n]; });
  document.querySelectorAll('[data-i18n-placeholder]').forEach(e => { if (L[e.dataset.i18nPlaceholder]) e.placeholder = L[e.dataset.i18nPlaceholder]; });
}
applyI18n();

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

// ---- Tabs ----
function switchTab(tab) {
  const isSteam = tab === 'steam';
  $('tabBtnSteam').className = 'tab' + (isSteam ? ' active' : '');
  $('tabBtnDiscord').className = 'tab' + (!isSteam ? ' active' : '');
  $('tabSteam').hidden = !isSteam;
  $('tabDiscord').hidden = isSteam;
  if (!isSteam) {
    loadDiscordState();
    loadDiscordQuests();
  }
}
$('tabBtnSteam').onclick = () => switchTab('steam');
$('tabBtnDiscord').onclick = () => switchTab('discord');

// ---- Steam Farmer UI ----
let formInit = false, S = null, showLogin = false;
const esc = s => String(s).replace(/[&<>"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));

function render(st) {
  S = st;
  const pill = (el, cls, txt) => { $(el).className = 'pill ' + cls; $(el).textContent = txt; };
  pill('pSteam', st.steamRunning ? 'ok' : 'bad', st.steamRunning ? L.steamOn : L.steamOff);
  pill('pAuth', st.loggedIn ? 'ok' : 'warn', st.loggedIn ? (st.personaName || (L.authOn + st.steamId)) : L.authOff);
  pill('pStatus', st.status === 'running' ? 'ok' : '', L[st.status] + (st.status==='running' && st.active.length ? ` · ${st.active.length}` : ''));
  const note = st.workerError ? 'Worker files: ' + st.workerError : st.note;
  $('note').hidden = !note; $('note').textContent = note || '';

  $('btnLoginShow').hidden = st.loggedIn;

  // Steam Account Bar
  const persona = st.personaName || (st.steamId ? `Steam ID: ${st.steamId}` : '');
  if (st.loggedIn) {
    $('steamPersonaName').textContent = persona;
    $('steamIdDisplay').textContent = st.personaName && st.steamId ? st.steamId : '';
    if (st.avatarUrl) {
      $('steamAvatar').src = st.avatarUrl;
      $('steamAvatar').hidden = false;
    } else {
      $('steamAvatar').hidden = true;
    }
    $('btnSteamRemoveAccount').hidden = false;
  } else {
    $('steamPersonaName').textContent = L.steamNotLoggedIn;
    $('steamIdDisplay').textContent = '';
    $('steamAvatar').hidden = true;
    $('btnSteamRemoveAccount').hidden = true;
  }

  // Steam accounts select
  const accs = Array.isArray(st.steamAccounts) ? st.steamAccounts : [];
  if (accs.length > 1) {
    $('steamAccountSelect').hidden = false;
    $('steamAccountSelect').innerHTML = accs.map(a => {
      const label = a.personaName ? `${a.personaName} (${a.steamId})` : a.steamId;
      return `<option value="${a.steamId}" ${a.steamId === st.steamId ? 'selected' : ''}>${esc(label)}</option>`;
    }).join('');
  } else {
    $('steamAccountSelect').hidden = true;
  }

  $('loginBox').hidden = st.loggedIn && !showLogin;

  if (!formInit) {
    formInit = true;
    $('strategy').value = st.settings.strategy || 'batch';
    $('maxParallel').value = st.settings.maxParallel || 32;
    $('pollMin').value = st.settings.pollMinutes || 5;
  }
  const rank = {active:0, queued:1, failed:2, done:3};
  const games = [...st.games].sort((a,b) => rank[a.state]-rank[b.state] || b.hours-a.hours);
  const sq = ($('steamSearch')?.value || '').trim().toLowerCase();
  $('games').innerHTML = games.map(g => {
    const hide = sq && !String(g.appId).toLowerCase().includes(sq) && !(g.name||'').toLowerCase().includes(sq);
    return `<tr class="${g.state}"${hide ? ' hidden' : ''}><td>${g.appId}</td><td>${esc(g.name||'?')}</td><td>${g.remaining}</td><td>${g.hours.toFixed(1)}</td><td><span class="tag ${g.state}">${L[g.state]}</span></td></tr>`;
  }).join('');
  const pending = st.games.filter(g => g.state !== 'done');
  let info = L.gamesInfo(pending.length, pending.reduce((a,g)=>a+g.remaining,0));
  if (st.nextPoll) info += ` · ${L.nextPoll} ${new Date(st.nextPoll).toLocaleTimeString()}`;
  $('gamesInfo').textContent = info;
}

function addLog(line) {
  const atEnd1 = $('log').scrollTop + $('log').clientHeight >= $('log').scrollHeight - 20;
  $('log').textContent += line + '\n';
  if (atEnd1) $('log').scrollTop = $('log').scrollHeight;

  const dqLog = $('dqLog');
  if (dqLog) {
    const atEnd2 = dqLog.scrollTop + dqLog.clientHeight >= dqLog.scrollHeight - 20;
    dqLog.textContent += line + '\n';
    if (atEnd2) dqLog.scrollTop = dqLog.scrollHeight;
  }
}

$('btnSteamAddAccount').onclick = () => {
  showLogin = !showLogin;
  if (S) render(S);
};
$('btnLoginShow').onclick = () => {
  showLogin = !showLogin;
  if (S) render(S);
};
$('btnSteamRemoveAccount').onclick = async () => {
  if (confirm(L.confirmRemove)) {
    await act(() => post('steam/remove', {steamId: S.steamId}), L.accountRemoved);
  }
};
$('steamAccountSelect').onchange = async () => {
  const sid = $('steamAccountSelect').value;
  if (sid && S && sid !== S.steamId) {
    await act(() => post('steam/switch', {steamId: sid}), L.accountSwitched);
  }
};

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
$('btnQuit').onclick = async () => {
  try { await post('quit'); } catch (e) {}
  document.body.innerHTML = '<p style="padding:2em">Stopped. You can close this window.</p>';
  window.close();
};
$('btnSave').onclick = () => act(() => post('settings', {strategy:$('strategy').value, maxParallel:+$('maxParallel').value, pollMinutes:+$('pollMin').value}).then(render), L.saved);
$('btnCk').onclick = async () => {
  $('ckErr').textContent = '';
  try {
    await post('login/cookies', {LoginSecure:$('ckLogin').value, SessionID:$('ckSess').value});
    showLogin = false;
    toast(L.cookiesOk, 'ok');
  } catch (e) {
    $('ckErr').textContent = e.message;
    toast(e.message, 'err');
  }
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
    if (q.status === 'done' || q.status === 'error') {
      clearInterval(qrTimer);
      if (q.status==='done') {
        showLogin = false;
        $('qrWrap').hidden = true;
      }
    }
  }, 1500);
};

// ---- Discord Quests Module ----
let DQ = { hasToken: false, running: [], quests: [], accounts: [], activeId: '' };
let showDiscordAdd = false;

async function loadDiscordState() {
  try {
    const st = await api('discord/state');
    DQ.hasToken = !!st.hasToken;
    DQ.running = Array.isArray(st.running) ? st.running : [];
    DQ.accounts = Array.isArray(st.accounts) ? st.accounts : [];
    DQ.activeId = st.activeId || '';
    updateDiscordHeader();
  } catch (e) {}
}

function updateDiscordHeader() {
  const accs = DQ.accounts;
  const active = accs.find(a => a.id === DQ.activeId) || accs[0];

  if (DQ.hasToken && active) {
    const displayName = active.globalName || active.username || ('User #' + active.id);
    $('discordPersonaName').textContent = displayName;
    $('discordIdDisplay').textContent = active.username && active.username !== displayName ? `@${active.username} · ${active.id}` : active.id;
    $('discordAvatar').src = active.avatarUrl || 'https://cdn.discordapp.com/embed/avatars/0.png';
    $('discordAvatar').hidden = false;
    $('btnDiscordRemoveAccount').hidden = false;
  } else {
    $('discordPersonaName').textContent = L.discordNotConnected;
    $('discordIdDisplay').textContent = '';
    $('discordAvatar').hidden = true;
    $('btnDiscordRemoveAccount').hidden = true;
  }

  if (accs.length > 1) {
    $('discordAccountSelect').hidden = false;
    $('discordAccountSelect').innerHTML = accs.map(a => {
      const label = a.globalName || a.username || a.id;
      return `<option value="${a.id}" ${a.id === DQ.activeId ? 'selected' : ''}>${esc(label)}</option>`;
    }).join('');
  } else {
    $('discordAccountSelect').hidden = true;
  }

  $('dqTokenBox').hidden = DQ.hasToken && !showDiscordAdd;
}

async function loadDiscordQuests() {
  try {
    const quests = await api('discord/quests');
    DQ.quests = Array.isArray(quests) ? quests : [];
    const banner = $('dqWarningBanner');
    if (banner) banner.hidden = true;
    renderDiscordQuests();
  } catch (e) {
    const banner = $('dqWarningBanner');
    if (DQ.quests && DQ.quests.length > 0) {
      if (banner) {
        banner.textContent = '⚠️ ' + e.message;
        banner.hidden = !e.message;
      }
    } else {
      $('dqQuestList').innerHTML = `<div class="empty-hint">${esc(e.message)}</div>`;
    }
  }
}

function formatTime(sec) {
  sec = Math.max(0, Math.round(sec));
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return String(m).padStart(2, '0') + ':' + String(s).padStart(2, '0');
}

function formatDuration(sec) {
  return formatTime(sec);
}

function buildQuestCard(q, isRunning) {
  const id = q.id;
  const name = esc(q.config?.messages?.quest_name || `Quest #${id}`);
  const taskName = q.task_name || q.config?.quest_task_config?.type || 'PLAY_ON_DESKTOP';
  const totalSec = q.seconds_target > 0 ? q.seconds_target : (q.config?.quest_task_config?.stream_duration_requirement_minutes || 15) * 60;
  const progSec = q.seconds_done || 0;

  const uStatus = q.user_status || {};
  const isEnrolled = !!uStatus.enrolled_at;
  const isClaimed = !!q.is_claimed || !!uStatus.claimed_at;
  const isCompleted = !!uStatus.completed_at || progSec >= totalSec;

  const pct = Math.min(100, Math.round((progSec / (totalSec || 1)) * 100));
  const rewardMsg = esc(q.config?.reward_config?.messages?.completed || '');
  const rewardName = q.reward_name || '';
  const rewardBadge = rewardName ? `<span class="reward-badge">✦ ${esc(rewardName)}</span>` : '';

  let enrollTag = isEnrolled
    ? `<span class="tag ok" style="margin-right:4px;">${L.dqEnrolled}</span>`
    : `<span class="tag" style="margin-right:4px;">${L.dqNotEnrolled}</span>`;

  let statusTag = '', actionBtn = '';
  const startBtnLabel = q.is_video ? L.dqWatch : L.dqStartQuest;
  if (isClaimed) {
    statusTag = `<span class="tag done">${L.dqClaimed}</span>`;
    actionBtn = '';
  } else if (isRunning) {
    statusTag = `<span class="tag active">${L.dqRunning}</span>`;
    actionBtn = `<button class="stop" onclick="stopDiscordQuest('${id}')">${L.dqStop}</button>`;
  } else if (isCompleted) {
    statusTag = `<span class="tag ok">${L.dqCompleted}</span>`;
    actionBtn = `<span class="ready-to-claim-badge">&#10003; ${L.dqReadyToClaim}</span>`;
  } else {
    statusTag = `<span class="tag">${L.dqAvailable}</span>`;
    const enrollBtn = !isEnrolled ? `<button class="secondary" style="margin-right:6px;" onclick="enrollDiscordQuest('${id}')">${L.dqEnrollBtn}</button>` : '';
    actionBtn = `${enrollBtn}<button class="go" onclick="startDiscordQuest('${id}')">${startBtnLabel}</button>`;
  }

  const cardClass = isRunning ? 'quest-card running' : (isClaimed ? 'quest-card done' : (isCompleted ? 'quest-card completed' : 'quest-card'));
  const typeLabel = q.is_video ? `${L.dqVideo} (${formatTime(totalSec)})` : `${taskName} · ${formatTime(totalSec)}`;

  return `<div class="${cardClass}" id="quest_${id}" data-name="${name.toLowerCase()}" data-task="${taskName.toLowerCase()}">
    <div class="quest-header">
      <div>
        <div class="quest-title-row"><span class="quest-title">${name}</span>${rewardBadge}</div>
        <div class="quest-sub">${typeLabel}${rewardMsg ? ' · ' + rewardMsg : ''}</div>
      </div>
      <div class="quest-badges">${enrollTag}${statusTag}${actionBtn}</div>
    </div>
    <div class="progress-track"><div class="progress-fill" style="width:${pct}%;"></div></div>
    <div class="progress-meta"><span>${formatTime(progSec)} / ${formatTime(totalSec)}</span><span>${pct}%</span></div>
  </div>`;
}

function renderDiscordQuests() {
  const filter = ($('dqSearch') || {value:''}).value.trim().toLowerCase();
  const list = $('dqQuestList');

  // Split active vs claimed (server now returns active first, claimed after)
  const active = DQ.quests.filter(q => !q.is_claimed && !(q.user_status?.claimed_at));
  const claimed = DQ.quests.filter(q => q.is_claimed || q.user_status?.claimed_at);

  const totalCount = DQ.quests.length;
  const claimedCount = claimed.length;
  const runningCount = DQ.running.length;

  if (active.length === 0 && !DQ.hasToken) {
    list.innerHTML = `<div class="empty-hint">${L.dqEmpty}</div>`;
    $('dqQuestsCount').textContent = '';
    $('dqSummary').textContent = '';
    $('dqClaimedSection').hidden = true;
    return;
  }

  // Active quests
  const filtered = active.filter(q => {
    if (!filter) return true;
    const name = (q.config?.messages?.quest_name || '').toLowerCase();
    const task = (q.task_name || '').toLowerCase();
    return name.includes(filter) || task.includes(filter);
  });

  if (filtered.length === 0) {
    list.innerHTML = `<div class="empty-hint">${filter ? L.dqNoResults : L.dqEmpty}</div>`;
  } else {
    list.innerHTML = filtered.map(q => buildQuestCard(q, DQ.running.includes(q.id))).join('');
  }

  $('dqQuestsCount').textContent = `(${active.length})`;

  // Dynamic counter: Running: N · Completed: X / Y
  const parts = [];
  if (runningCount > 0) parts.push(`${L.dqRunning}: ${runningCount}`);
  parts.push(`${L.dqClaimed}: ${claimedCount} / ${totalCount}`);
  $('dqSummary').textContent = parts.join(' · ');

  // Claimed section
  const claimedSection = $('dqClaimedSection');
  const claimedList = $('dqClaimedList');
  if (claimed.length > 0) {
    claimedSection.hidden = false;
    // Update summary label
    const summary = claimedSection.querySelector('.claimed-summary');
    if (summary) summary.textContent = `${L.dqClaimedSection} (${claimed.length})`;
    claimedList.innerHTML = claimed.map(q => buildQuestCard(q, false)).join('');
  } else {
    claimedSection.hidden = true;
  }
}

// Live search for Discord quests
$('dqSearch').oninput = () => renderDiscordQuests();

// Live search for Steam games
$('steamSearch').oninput = function() {
  const q = this.value.trim().toLowerCase();
  document.querySelectorAll('#games tr').forEach(tr => {
    const appId = (tr.children[0]?.textContent || '').toLowerCase();
    const name = (tr.children[1]?.textContent || '').toLowerCase();
    tr.hidden = q ? !(appId.includes(q) || name.includes(q)) : false;
  });
};

function getDiscordMode() {
  return ($('dqModeSelect') || {}).value || localStorage.getItem('xmk_dq_mode') || 'safe';
}

if ($('dqModeSelect')) {
  const savedMode = localStorage.getItem('xmk_dq_mode') || 'safe';
  $('dqModeSelect').value = savedMode;
  if ($('dqFastWarning')) $('dqFastWarning').hidden = (savedMode !== 'fast');
  $('dqModeSelect').onchange = function() {
    localStorage.setItem('xmk_dq_mode', this.value);
    if ($('dqFastWarning')) $('dqFastWarning').hidden = (this.value !== 'fast');
  };
}

window.enrollDiscordQuest = async (id) => {
  await act(() => post('discord/enroll', {questId: id}), L.dqEnrolledSuccess);
  await loadDiscordQuests();
};

window.startDiscordQuest = async (id) => {
  const mode = getDiscordMode();
  await act(() => post('discord/start', {questId: id, mode: mode}), L.dqStarted);
  await loadDiscordState();
};

window.stopDiscordQuest = async (id) => {
  await act(() => post('discord/stop', {questId: id}), L.dqStopped);
  await loadDiscordState();
};

$('btnDiscordAddAccount').onclick = () => {
  showDiscordAdd = !showDiscordAdd;
  updateDiscordHeader();
};

$('btnDiscordRemoveAccount').onclick = async () => {
  if (confirm(L.confirmRemove)) {
    await act(() => post('discord/remove', {id: DQ.activeId}), L.accountRemoved);
    await loadDiscordState();
    await loadDiscordQuests();
  }
};

$('discordAccountSelect').onchange = async () => {
  const newId = $('discordAccountSelect').value;
  if (newId && newId !== DQ.activeId) {
    await act(() => post('discord/switch', {id: newId}), L.accountSwitched);
    await loadDiscordState();
    await loadDiscordQuests();
  }
};

$('dqBtnSaveToken').onclick = async () => {
  const token = $('dqTokenInput').value.trim();
  if (!token) return toast('Please enter a Discord token', 'err');
  await act(async () => {
    await post('discord/token', {token});
    showDiscordAdd = false;
    $('dqTokenInput').value = '';
    await loadDiscordState();
    await loadDiscordQuests();
  }, L.dqSaved);
};

$('dqBtnHowTo').onclick = () => {
  $('dqHowToBox').hidden = !$('dqHowToBox').hidden;
};

$('dqBtnStartAll').onclick = async () => {
  const mode = getDiscordMode();
  await act(() => post('discord/start', {all: true, mode: mode}), L.dqStarted);
  await loadDiscordState();
};

$('dqBtnStopAll').onclick = async () => {
  await act(() => post('discord/stop'), L.dqStopped);
  await loadDiscordState();
};

$('dqBtnRefresh').onclick = async () => {
  await act(async () => {
    await loadDiscordState();
    await loadDiscordQuests();
  }, L.refreshing);
};

// Startup initialization
api('state').then(render).catch(e => toast(e.message, 'err'));
api('logs').then(ls => {
  ls = Array.isArray(ls) ? ls : [];
  const text = ls.join('\n') + (ls.length ? '\n' : '');
  $('log').textContent = text;
  $('log').scrollTop = 1e9;
  if ($('dqLog')) {
    $('dqLog').textContent = text;
    $('dqLog').scrollTop = 1e9;
  }
}).catch(() => {});

loadDiscordState();

try {
  const es = new EventSource('/api/events');
  es.addEventListener('state', e => { try { render(JSON.parse(e.data)); } catch (x) {} });
  es.addEventListener('log', e => addLog(e.data));
  es.addEventListener('discord', e => {
    try {
      const st = JSON.parse(e.data);
      DQ.hasToken = !!st.hasToken;
      DQ.running = Array.isArray(st.running) ? st.running : [];
      DQ.accounts = Array.isArray(st.accounts) ? st.accounts : [];
      DQ.activeId = st.activeId || '';

      // Update real-time progress for all running quests without hitting Discord API
      if (st.progress && Array.isArray(DQ.quests)) {
        for (const q of DQ.quests) {
          if (st.progress[q.id]) {
            q.seconds_done = st.progress[q.id].elapsed;
            q.seconds_target = st.progress[q.id].total;
            q.progress_ratio = q.seconds_done / (q.seconds_target || 1);
          }
        }
      }

      updateDiscordHeader();
      renderDiscordQuests();

      // Only refresh quest list from server when a quest finishes!
      if (st.lastCompleted && st.lastCompleted !== DQ.lastCompleted) {
        DQ.lastCompleted = st.lastCompleted;
        loadDiscordQuests();
      }
    } catch (x) {}
  });
  es.onerror = () => { $('pStatus').className = 'pill bad'; $('pStatus').textContent = L.lost; };
} catch (e) { toast(e.message, 'err'); }

setInterval(() => {
  api('state').then(render).catch(() => {});
}, 10000);
