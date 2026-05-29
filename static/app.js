// Crypto Price Alerts — frontend

const alertsBody = document.getElementById('alerts-body');
const alertsCards = document.getElementById('alerts-cards');
const formHorizontal = document.getElementById('form-horizontal');
const formChannel = document.getElementById('form-channel');
const tabButtons = document.querySelectorAll('.tab-btn');
const panelHorizontal = document.getElementById('panel-horizontal');
const panelChannel = document.getElementById('panel-channel');
const tzHint = document.getElementById('tz-hint');
const addAlertToggle = document.getElementById('toggle-add-alert');
const addAlertBody = document.querySelector('.card-form-body');
const settingsToggle = document.getElementById('toggle-settings');
const settingsBody = document.querySelector('.card-settings-body');
const updateButton = document.getElementById('btn-update-app');
const updateStatus = document.getElementById('update-status');

const addIcon = '<span class="material-symbols-outlined" aria-hidden="true">add</span>';
const settingsIcon = '<span class="material-symbols-outlined" aria-hidden="true">settings</span>';
const closeIcon = '<span class="material-symbols-outlined" aria-hidden="true">close</span>';

let appTimezone = 'Europe/Istanbul';
let appTouchTolerancePercent = 0.15;

async function loadAppConfig() {
  try {
    const cfg = await api('/api/config');
    if (cfg && cfg.utc) {
      appTimezone = cfg.utc;
    }
  } catch (_) {
    /* keep default */
  }
  if (tzHint) {
    tzHint.textContent = 'Datetimes use timezone: ' + appTimezone;
  }
}

function fmtTime(iso) {
  if (!iso || iso === '0001-01-01T00:00:00Z') return '—';
  const d = new Date(iso);
  if (isNaN(d)) return '—';
  return new Intl.DateTimeFormat(undefined, {
    timeZone: appTimezone,
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(d);
}

function typeLabel(type) {
  return type === 'channel' ? 'diagonal' : type;
}

function unixToDatetimeLocal(unix) {
  if (!unix) return '';
  const s = new Date(unix * 1000)
    .toLocaleString('sv-SE', { timeZone: appTimezone })
    .replace(' ', 'T');
  return s.slice(0, 16);
}

function fmtPrice(n) {
  if (n == null || n === undefined) return '—';
  return Number(n).toFixed(4);
}

function fmtPercent(n) {
  if (n == null || n === undefined) return '';
  return Number(n).toFixed(2) + '%';
}

function boundPercent(bound, currentPrice) {
  if (bound == null || currentPrice == null) return null;
  if (bound === 0) return null;
  return (Math.abs(currentPrice - bound) / Math.abs(bound)) * 100;
}

function boundCellHtml(bound, currentPrice) {
  const percent = boundPercent(bound, currentPrice);
  const near = percent != null && appTouchTolerancePercent != null && percent <= appTouchTolerancePercent;
  return (
    '<td class="bound' + (near ? ' near' : '') + '">' +
    fmtPrice(bound) +
    (percent != null ? ' <span class="bound-percent">(' + fmtPercent(percent) + ')</span>' : '') +
    '</td>'
  );
}

function boundCardHtml(label, bound, currentPrice) {
  const percent = boundPercent(bound, currentPrice);
  const near = percent != null && appTouchTolerancePercent != null && percent <= appTouchTolerancePercent;
  return (
    '<div>' + label + ' <span class="bound' + (near ? ' near' : '') + '">' +
    fmtPrice(bound) +
    (percent != null ? ' <span class="bound-percent">(' + fmtPercent(percent) + ')</span>' : '') +
    '</span></div>'
  );
}

function boundsCells(a) {
  if (a.type !== 'channel') {
    return '<td>—</td><td>—</td>';
  }
  return boundCellHtml(a.current_lower, a.current_price) + boundCellHtml(a.current_upper, a.current_price);
}

function boundsCardRows(a) {
  if (a.type !== 'channel') return '';
  return boundCardHtml('Lower', a.current_lower, a.current_price) + boundCardHtml('Upper', a.current_upper, a.current_price);
}

function setSectionOpen(open, body, toggle, { openLabel = 'Show', closeLabel = 'Hide' } = {}) {
  if (!body || !toggle) return;
  body.hidden = !open;
  toggle.innerHTML = open ? closeLabel : openLabel;
  toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
}

function setAddAlertOpen(open) {
  setSectionOpen(open, addAlertBody, addAlertToggle, { openLabel: addIcon, closeLabel: closeIcon });
}

function setSettingsOpen(open) {
  setSectionOpen(open, settingsBody, settingsToggle, { openLabel: settingsIcon, closeLabel: closeIcon });
}

function switchTab(tab, scrollToForm) {
  const isHorizontal = tab === 'horizontal';

  tabButtons.forEach((btn) => {
    const active = btn.dataset.tab === tab;
    btn.classList.toggle('active', active);
    btn.setAttribute('aria-selected', active ? 'true' : 'false');
  });

  panelHorizontal.classList.toggle('active', isHorizontal);
  panelHorizontal.hidden = !isHorizontal;
  panelChannel.classList.toggle('active', !isHorizontal);
  panelChannel.hidden = isHorizontal;

  if (scrollToForm) {
    document.querySelector('.card-form')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }
}

tabButtons.forEach((btn) => {
  btn.addEventListener('click', () => switchTab(btn.dataset.tab));
});

if (addAlertToggle) {
  addAlertToggle.addEventListener('click', () => {
    setAddAlertOpen(addAlertBody ? addAlertBody.hidden : false);
  });
}

if (settingsToggle) {
  settingsToggle.addEventListener('click', () => {
    setSettingsOpen(settingsBody ? settingsBody.hidden : false);
  });
}

setAddAlertOpen(false);
setSettingsOpen(false);

async function api(path, options = {}) {
  const token = localStorage.getItem('notifier_token');
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = 'Bearer ' + token;
  const res = await fetch(path, {
    headers,
    ...options,
  });
  if (res.status === 204) return null;
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

async function updateApp() {
  if (!updateButton) return;
  if (!confirm('Update pricealert to the latest version?')) return;

  updateButton.disabled = true;
  const originalText = updateButton.textContent;
  updateButton.textContent = 'Updating…';
  if (updateStatus) {
    updateStatus.textContent = '';
  }

  try {
    const result = await api('/api/update', { method: 'POST' });
    if (updateStatus) {
      updateStatus.textContent = result.message || 'Update completed successfully.';
    }
  } catch (err) {
    if (updateStatus) {
      updateStatus.textContent = 'Update failed: ' + err.message;
    } else {
      alert('Update failed: ' + err.message);
    }
  } finally {
    updateButton.disabled = false;
    updateButton.textContent = originalText;
  }
}

if (updateButton) {
  updateButton.addEventListener('click', updateApp);
}

// Login flow
const loginPanel = document.getElementById('login-panel');
const formLogin = document.getElementById('form-login');

function showLogin() {
  if (loginPanel) loginPanel.hidden = false;
}
function hideLogin() {
  if (loginPanel) loginPanel.hidden = true;
}

if (formLogin) {
  formLogin.addEventListener('submit', async (e) => {
    e.preventDefault();
    const secret = formLogin.secret.value.trim();
    if (!secret) return;
    try {
      const res = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ secret }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'login failed');
      localStorage.setItem('notifier_token', data.token);
      hideLogin();
      await loadAppConfig();
      loadAlerts();
    } catch (err) {
      alert('Login failed: ' + err.message);
    }
  });
}

async function loadAlerts() {
  try {
    const alerts = await api('/api/alerts');
    renderAlerts(alerts || []);
  } catch (e) {
    const msg = escapeHtml(e.message);
    alertsBody.innerHTML =
      '<tr><td colspan="9" class="empty">Error: ' + msg + '</td></tr>';
    alertsCards.innerHTML = '<p class="empty">Error: ' + msg + '</p>';
  }
}

function escapeHtml(s) {
  const div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}










function actionButtons(a) {
  return `
    <button class=" btn-none" data-edit="${escapeHtml(a.id)}" aria-label="Edit alert">
      <span class="material-symbols-outlined">edit</span>
    </button>
    <button class=" btn-none" data-delete="${escapeHtml(a.id)}" aria-label="Delete alert">
      <span class="material-symbols-outlined">delete</span>
    </button>`;
}





function toggleCardDetails(card) {
  if (!card) return;
  const expanded = card.classList.toggle('expanded');
  card.setAttribute('aria-expanded', expanded ? 'true' : 'false');
}

function bindActions(container) {
  container.querySelectorAll('[data-edit]').forEach((btn) => {
    btn.addEventListener('click', () => editAlert(btn.dataset.edit));
  });
  container.querySelectorAll('[data-toggle]').forEach((btn) => {
    btn.addEventListener('click', () => toggleAlert(btn.dataset.toggle));
  });
  container.querySelectorAll('[data-delete]').forEach((btn) => {
    btn.addEventListener('click', () => deleteAlert(btn.dataset.delete));
  });
  container.querySelectorAll('.alert-card').forEach((card) => {
    card.addEventListener('keydown', (event) => {
      if (event.code === 'Space' || event.key === ' ' || event.key === 'Spacebar') {
        event.preventDefault();
        toggleCardDetails(card);
      }
    });
    card.addEventListener('click', (event) => {
      if (event.target.closest('button, [data-edit], [data-delete], [data-toggle]')) return;
      toggleCardDetails(card);
    });
  });
}

function alertCloseness(a) {
  if (a.type !== 'channel' || a.current_price == null || a.current_lower == null || a.current_upper == null) {
    return Infinity;
  }
  const lowerPct = boundPercent(a.current_lower, a.current_price);
  const upperPct = boundPercent(a.current_upper, a.current_price);
  return Math.min(lowerPct || Infinity, upperPct || Infinity);
}

function sortAlerts(alerts) {
  return alerts.slice().sort((a, b) => {
    const aClose = alertCloseness(a);
    const bClose = alertCloseness(b);
    if (aClose !== bClose) {
      return aClose - bClose;
    }
    const aDate = new Date(a.created_at).getTime();
    const bDate = new Date(b.created_at).getTime();
    return bDate - aDate;
  });
}

function renderAlerts(alerts) {
  alerts = sortAlerts(alerts);
  if (!alerts.length) {
    alertsBody.innerHTML = '<tr><td colspan="9" class="empty">No alerts yet</td></tr>';
    alertsCards.innerHTML = '<p class="empty">No alerts yet</p>';
    return;
  }

  alertsBody.innerHTML = alerts
    .map((a) => {
      const shortId = a.id.length > 12 ? '…' + a.id.slice(-8) : a.id;
      const enabled = a.enabled
        ? '<span class="badge on">Yes</span>'
        : '<span class="badge off">No</span>';
      return `
        <tr data-id="${escapeHtml(a.id)}">
          <td class="id-cell" title="${escapeHtml(a.id)}">${escapeHtml(shortId)}</td>
          <td>${escapeHtml(a.pair)}</td>
          <td class="type-badge">${escapeHtml(typeLabel(a.type))}</td>
          <td>${enabled}</td>
          ${boundsCells(a)}
          <td>${escapeHtml(fmtTime(a.created_at))}</td>
          <td>${escapeHtml(fmtTime(a.last_trigger))}</td>
          <td class="actions">${actionButtons(a)}</td>
        </tr>`;
    })
    .join('');

  alertsCards.innerHTML = alerts
    .map((a) => {
      const enabled = a.enabled
        ? '<span class="badge on">On</span>'
        : '<span class="badge off">Off</span>';
      return `
        <article class="alert-card" data-id="${escapeHtml(a.id)}" tabindex="0" role="button" aria-expanded="false">
          <div class="alert-card-header">
            <strong>${escapeHtml(a.pair)} <span class="secondary">${escapeHtml(a.label)}</span></strong>
            <div>
              <button class="btn-none" data-edit="${escapeHtml(a.id)}" aria-label="Edit alert">
                <span class="material-symbols-outlined">edit</span>
              </button>
              <button class="btn-none" data-delete="${escapeHtml(a.id)}" aria-label="Delete alert">
                <span class="material-symbols-outlined">delete</span>
              </button>
              <button class="btn-none" data-toggle="${escapeHtml(a.id)}" aria-label="${a.enabled ? 'Disable' : 'Enable'} alert">${enabled}</button>
            </div>
          </div>
          <div class="alert-card-meta">
            <!--<div>Type <span class="type-badge">${escapeHtml(typeLabel(a.type))}</span></div>-->
            ${boundsCardRows(a)}
            <div class="toggle-details">Created <span>${escapeHtml(fmtTime(a.created_at))}</span></div>
            <div class="toggle-details">Triggered <span>${escapeHtml(fmtTime(a.last_trigger))}</span></div>
          </div>
        </article>`;
    })
    .join('');

  bindActions(alertsBody);
  bindActions(alertsCards);
}

async function editAlert(id) {
  const alerts = await api('/api/alerts');
  const a = alerts.find((x) => x.id === id);
  if (!a) return;

  if (a.type === 'horizontal') {
    switchTab('horizontal', true);
    setAddAlertOpen(true);
    const f = formHorizontal;
    f.edit_id.value = a.id;
    f.pair.value = a.pair;
    f.target_price.value = a.target_price;
    f.condition.value = a.condition;
    f.label.value = a.label || '';
    f.querySelector('[data-cancel="horizontal"]').hidden = false;
  } else {
    switchTab('channel', true);
    setAddAlertOpen(true);
    const f = formChannel;
    f.edit_id.value = a.id;
    f.pair.value = a.pair;
    f.p1_datetime.value = a.p1_datetime || unixToDatetimeLocal(a.p1_time);
    f.p1_price.value = a.p1_price;
    f.p2_datetime.value = a.p2_datetime || unixToDatetimeLocal(a.p2_time);
    f.p2_price.value = a.p2_price;
    f.offset.value = a.offset;
    f.trigger_side.value = a.trigger_side;
    f.label.value = a.label || '';
    f.querySelector('[data-cancel="channel"]').hidden = false;
  }
}

function resetForm(form, cancelKey) {
  form.reset();
  form.edit_id.value = '';
  form.querySelector(`[data-cancel="${cancelKey}"]`).hidden = true;
}

async function toggleAlert(id) {
  await api('/api/alerts/' + id + '/toggle', { method: 'POST' });
  loadAlerts();
}

async function deleteAlert(id) {
  if (!confirm('Delete this alert?')) return;
  await api('/api/alerts/' + id, { method: 'DELETE' });
  loadAlerts();
}

formHorizontal.addEventListener('submit', async (e) => {
  e.preventDefault();
  const f = formHorizontal;
  const body = {
    pair: f.pair.value.trim().toUpperCase(),
    type: 'horizontal',
    target_price: parseFloat(f.target_price.value),
    condition: f.condition.value,
    label: f.label.value.trim(),
  };
  try {
    if (f.edit_id.value) {
      await api('/api/alerts/' + f.edit_id.value, {
        method: 'PUT',
        body: JSON.stringify(body),
      });
    } else {
      await api('/api/alerts', { method: 'POST', body: JSON.stringify(body) });
    }
    resetForm(f, 'horizontal');
    loadAlerts();
  } catch (err) {
    alert(err.message);
  }
});

formChannel.addEventListener('submit', async (e) => {
  e.preventDefault();
  const f = formChannel;
  const body = {
    pair: f.pair.value.trim().toUpperCase(),
    type: 'channel',
    p1_datetime: f.p1_datetime.value,
    p1_price: parseFloat(f.p1_price.value),
    p2_datetime: f.p2_datetime.value,
    p2_price: parseFloat(f.p2_price.value),
    offset: parseFloat(f.offset.value),
    trigger_side: f.trigger_side.value,
    label: f.label.value.trim(),
  };
  try {
    if (f.edit_id.value) {
      await api('/api/alerts/' + f.edit_id.value, {
        method: 'PUT',
        body: JSON.stringify(body),
      });
    } else {
      await api('/api/alerts', { method: 'POST', body: JSON.stringify(body) });
    }
    resetForm(f, 'channel');
    loadAlerts();
  } catch (err) {
    alert(err.message);
  }
});

document.querySelectorAll('[data-cancel]').forEach((btn) => {
  btn.addEventListener('click', () => {
    const key = btn.dataset.cancel;
    resetForm(key === 'horizontal' ? formHorizontal : formChannel, key);
  });
});

(async function init() {
  // If no token present, show login overlay; otherwise proceed.
  const token = localStorage.getItem('notifier_token');
  if (!token) {
    showLogin();
    return;
  }
  try {
    await loadAppConfig();
  } catch (_) {}
  loadAlerts();
  setInterval(loadAlerts, 10000);

  // Load settings into form
  try {
    const s = await api('/api/settings');
    const f = document.getElementById('form-settings');
    if (f && s) {
      f.utc.value = s.utc || '';
      f.telegram_token.value = s.telegram_token || '';
      f.telegram_chat_id.value = s.telegram_chat_id || '';
      f.touch_tolerance_percent.value = s.touch_tolerance_percent || '';
      f.poll_interval_sec.value = s.poll_interval_sec || '';
      if (s.touch_tolerance_percent != null) {
        appTouchTolerancePercent = parseFloat(s.touch_tolerance_percent) || appTouchTolerancePercent;
      }
    }
  } catch (_) {}

  const settingsForm = document.getElementById('form-settings');
  if (settingsForm) {
    settingsForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const f = settingsForm;
      const body = {
        utc: f.utc.value.trim(),
        telegram_token: f.telegram_token.value.trim(),
        telegram_chat_id: f.telegram_chat_id.value.trim(),
        touch_tolerance_percent: parseFloat(f.touch_tolerance_percent.value) || undefined,
        poll_interval_sec: parseInt(f.poll_interval_sec.value) || undefined,
      };
      try {
        await api('/api/settings', { method: 'PUT', body: JSON.stringify(body) });
        const savedTolerance = parseFloat(body.touch_tolerance_percent);
        if (!Number.isNaN(savedTolerance)) {
          appTouchTolerancePercent = savedTolerance;
        }
        alert('Settings saved');
        await loadAppConfig();
      } catch (err) {
        alert('Failed to save: ' + err.message);
      }
    });
  }
}());
