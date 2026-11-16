// Interfaz del sistema de parqueo. Consume la API REST de /api/v1.
'use strict';

const $ = (sel) => document.querySelector(sel);
const TZ = 'America/Costa_Rica';
const METHODS = { cash: 'Efectivo', card: 'Tarjeta', transfer: 'SINPE / transferencia', waived: 'Sin cobro', abonnement: 'Abono' };

const state = {
  token: null,
  user: null,
  site: null,
  types: new Map(),   // id -> tipo de vehículo
  zones: [],
  spaces: new Map(),  // id -> espacio
  open: [],           // tickets abiertos
  quote: null,        // cobro en curso
};

// ---------- Utilidades ----------

function storage(op, key, value) {
  try {
    if (op === 'get') return sessionStorage.getItem(key);
    if (op === 'set') sessionStorage.setItem(key, value);
    if (op === 'del') sessionStorage.removeItem(key);
  } catch (_) { /* almacenamiento no disponible */ }
  return null;
}

const normalizePlate = (p) => p.toUpperCase().replace(/[^A-Z0-9]/g, '');

function money(cents) {
  const colones = Math.round((cents || 0) / 100);
  return '₡' + colones.toLocaleString('es-CR');
}

function fmtTime(iso) {
  return new Date(iso).toLocaleString('es-CR', { timeZone: TZ, hour: '2-digit', minute: '2-digit', day: '2-digit', month: 'short' });
}

function duration(fromIso, toIso) {
  const mins = Math.max(0, Math.floor((new Date(toIso || Date.now()) - new Date(fromIso)) / 60000));
  const h = Math.floor(mins / 60);
  return h ? `${h} h ${mins % 60} min` : `${mins} min`;
}

function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

let toastTimer;
function toast(msg) {
  const t = $('#toast');
  t.textContent = msg;
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { t.hidden = true; }, 3000);
}

function showError(el, msg) {
  el.textContent = msg ? msg.charAt(0).toUpperCase() + msg.slice(1) : '';
  el.hidden = !msg;
}

const typeName = (id) => state.types.get(id)?.Name || '—';
const spaceCode = (id) => (id && state.spaces.get(id)?.Code) || '—';

// ---------- API ----------

async function api(method, path, body) {
  const headers = { 'Content-Type': 'application/json' };
  if (state.token) headers.Authorization = 'Bearer ' + state.token;
  const res = await fetch('/api/v1' + path, { method, headers, body: body ? JSON.stringify(body) : undefined });
  const data = await res.json().catch(() => null);
  if (res.status === 401 && state.token) {
    logout();
    throw new Error('La sesión expiró. Ingrese de nuevo.');
  }
  if (!res.ok) throw new Error(data?.message || `Error ${res.status}`);
  return data;
}

// ---------- Sesión ----------

async function login(username, password) {
  const res = await api('POST', '/auth/login', { username, password });
  state.token = res.access_token;
  storage('set', 'token', state.token);
  await start();
}

function logout() {
  state.token = null;
  storage('del', 'token');
  $('#app-view').hidden = true;
  $('#login-view').hidden = false;
}

async function start() {
  state.user = await api('GET', '/auth/me');
  const sites = await api('GET', '/catalog/sites');
  state.site = sites.find((s) => s.ID === state.user.site_id) || sites[0];
  if (!state.site) throw new Error('No hay sedes configuradas.');

  const types = await api('GET', '/catalog/vehicle-types');
  state.types = new Map(types.filter((t) => t.IsActive).map((t) => [t.ID, t]));

  state.zones = (await api('GET', `/catalog/zones?site_id=${state.site.ID}`)).filter((z) => z.IsActive);

  $('#site-name').textContent = state.site.Name;
  $('#user-label').textContent = `${state.user.username} · ${roleName(state.user.role)}`;
  $('#entry-type').innerHTML = [...state.types.values()].map((t) => `<option value="${t.ID}">${esc(t.Name)}</option>`).join('');
  document.querySelectorAll('[data-roles]').forEach((b) => {
    b.hidden = !b.dataset.roles.split(',').includes(state.user.role);
  });

  $('#login-view').hidden = true;
  $('#app-view').hidden = false;
  await refresh();
  loadRates();
}

const roleName = (r) => ({ admin: 'Administrador', operator: 'Operador', auditor: 'Auditor' }[r] || r);

// ---------- Datos en pantalla ----------

async function refresh() {
  if (!state.site) return;
  const [occ, open, spaceLists] = await Promise.all([
    api('GET', `/reports/occupancy?site_id=${state.site.ID}`),
    api('GET', `/tickets?site_id=${state.site.ID}&status=open&limit=200`),
    Promise.all(state.zones.map((z) => api('GET', `/catalog/spaces?zone_id=${z.ID}&limit=200`))),
  ]);
  state.open = open.data || [];
  state.spaces = new Map();
  spaceLists.forEach((l) => (l.data || []).forEach((s) => state.spaces.set(s.ID, s)));

  $('#st-free').textContent = occ.FreeSpaces;
  $('#st-occ').textContent = occ.OccupiedSpaces;
  $('#st-pct').textContent = Math.round(occ.OccupancyPct) + '%';
  $('#st-bar').style.width = Math.min(100, occ.OccupancyPct) + '%';
  $('#st-in').textContent = state.open.length;

  renderMap();
  renderInside();
  renderSpaceOptions();
  $('#plates-inside').innerHTML = state.open.map((t) => `<option value="${esc(t.PlateNormalized)}">`).join('');
}

function renderSpaceOptions() {
  const type = state.types.get($('#entry-type').value);
  const sel = $('#entry-space');
  const current = sel.value;
  const free = [...state.spaces.values()].filter((s) => s.IsActive && s.Status === 'free' && (!type || s.Type === type.Code));
  sel.innerHTML = `<option value="">Asignar automáticamente (${free.length} libres)</option>` +
    free.map((s) => `<option value="${s.ID}">${esc(s.Code)}</option>`).join('');
  if (free.some((s) => s.ID === current)) sel.value = current;
}

function renderMap() {
  const byspace = new Map(state.open.filter((t) => t.SpaceID).map((t) => [t.SpaceID, t]));
  $('#map').innerHTML = state.zones.map((z) => {
    const spaces = [...state.spaces.values()].filter((s) => s.ZoneID === z.ID);
    return `<div class="zone"><h3>${esc(z.Name)} · ${spaces.length} espacios</h3><div class="spaces">` +
      spaces.map((s) => {
        const t = byspace.get(s.ID);
        const cls = t || s.Status === 'occupied' ? 'occupied' : (s.Status === 'free' && s.IsActive ? '' : 'other');
        return `<div class="space ${cls}" ${t ? `data-plate="${esc(t.PlateNormalized)}" title="Entrada: ${esc(fmtTime(t.EntryAt))}"` : ''}>` +
          `${esc(s.Code)}<small>${t ? esc(t.PlateNormalized) : ''}</small></div>`;
      }).join('') + '</div></div>';
  }).join('') || '<p class="empty">No hay zonas configuradas.</p>';
}

function renderInside() {
  const rows = [...state.open].sort((a, b) => new Date(a.EntryAt) - new Date(b.EntryAt)).map((t) => `
    <tr>
      <td><strong>${esc(t.PlateNormalized)}</strong>${t.SubscriptionID ? ' <span class="badge ok">Abonado</span>' : ''}</td>
      <td>${esc(typeName(t.TypeID))}</td>
      <td>${esc(spaceCode(t.SpaceID))}</td>
      <td>${esc(fmtTime(t.EntryAt))}</td>
      <td>${duration(t.EntryAt)}</td>
      <td><button class="btn small" data-charge="${esc(t.PlateNormalized)}">Cobrar</button></td>
    </tr>`).join('');
  $('#inside-body').innerHTML = rows || '<tr><td colspan="6" class="empty">No hay vehículos dentro.</td></tr>';
}

// ---------- Entrada ----------

async function registerEntry(form) {
  const raw = form.plate.value.trim();
  const plate = normalizePlate(raw);
  showError($('#entry-error'), '');
  if (plate.length < 3) return showError($('#entry-error'), 'Escriba una placa válida.');

  const body = { site_id: state.site.ID, plate_normalized: plate, plate_raw: raw.toUpperCase(), type_id: form.type.value };
  if (form.space.value) body.space_id = form.space.value;
  const t = await api('POST', '/tickets', body);
  await refresh();

  $('#entry-result').innerHTML = `
    <h2>Entrada registrada</h2>
    <p class="muted">Espacio asignado</p>
    <div class="big-space">${esc(spaceCode(t.SpaceID))}</div>
    <dl class="kv">
      <dt>Placa</dt><dd>${esc(t.PlateNormalized)}</dd>
      <dt>Tipo</dt><dd>${esc(typeName(t.TypeID))}</dd>
      <dt>Hora de entrada</dt><dd>${esc(fmtTime(t.EntryAt))}</dd>
      <dt>Condición</dt><dd>${t.SubscriptionID ? '<span class="badge ok">Abonado, no paga</span>' : 'Cobro por hora'}</dd>
    </dl>`;
  form.plate.value = '';
  form.space.value = '';
  form.plate.focus();
  toast(`Entrada de ${t.PlateNormalized} en ${spaceCode(t.SpaceID)}`);
}

// ---------- Salida y cobro ----------

async function quoteByPlate(rawPlate) {
  const plate = normalizePlate(rawPlate);
  showError($('#exit-error'), '');
  await refresh();
  const t = state.open.find((x) => x.PlateNormalized === plate);
  if (!t) {
    state.quote = null;
    $('#quote-card').innerHTML = '<h2>Cobro</h2><p class="muted">Busque una placa para calcular el monto.</p>';
    return showError($('#exit-error'), `No hay ningún vehículo con placa ${plate} dentro del parqueo.`);
  }
  const q = await api('POST', `/tickets/${t.ID}/quote`);
  state.quote = { ticket: t, quote: q, at: new Date().toISOString() };
  renderQuote();
}

function renderQuote() {
  const { ticket: t, quote: q, at } = state.quote;
  const total = q.Amount;
  const free = total === 0;
  $('#quote-card').innerHTML = `
    <h2>Cobro · ${esc(t.PlateNormalized)}</h2>
    <dl class="kv">
      <dt>Espacio</dt><dd>${esc(spaceCode(t.SpaceID))}</dd>
      <dt>Entrada</dt><dd>${esc(fmtTime(t.EntryAt))}</dd>
      <dt>Tiempo</dt><dd>${duration(t.EntryAt, at)}</dd>
    </dl>
    <ul class="breakdown">${(q.Breakdown || []).map((b) => `<li><span>${esc(b.Concept)}${b.Minutes ? ` (${b.Minutes} min)` : ''}</span><span>${money(b.Amount)}</span></li>`).join('')}</ul>
    <p class="muted">Total a pagar</p>
    <div class="total">${money(total)}</div>
    <form id="pay-form">
      ${free ? '' : `
      <div class="pay-row">
        <label>Método<select name="method">
          <option value="cash">Efectivo</option><option value="card">Tarjeta</option><option value="transfer">SINPE / transferencia</option>
        </select></label>
        <label id="received-wrap">Recibido (₡)<input name="received" type="number" min="0" step="5" value="${Math.round(total / 100)}"></label>
      </div>
      <p class="change" id="change"></p>`}
      <button class="btn primary" type="submit">${free ? 'Registrar salida sin cobro' : 'Cobrar y registrar salida'}</button>
    </form>`;
  if (!free) updateChange();
}

function updateChange() {
  const form = $('#pay-form');
  if (!form || !form.method) return;
  const cash = form.method.value === 'cash';
  $('#received-wrap').hidden = !cash;
  const received = Math.round(Number(form.received.value || 0) * 100);
  const diff = received - state.quote.quote.Amount;
  $('#change').innerHTML = !cash ? '' : diff >= 0
    ? `Vuelto: <strong>${money(diff)}</strong>`
    : `<span class="error">Faltan ${money(-diff)}</span>`;
}

async function settle(form) {
  const { ticket: t, quote: q } = state.quote;
  const total = q.Amount;
  let method = total === 0 ? (t.SubscriptionID ? 'abonnement' : 'waived') : form.method.value;
  let received = total;
  if (method === 'cash') {
    received = Math.round(Number(form.received.value || 0) * 100);
    if (received < total) return toast('El monto recibido no alcanza.');
  }
  // Se registra el total; el vuelto se entrega en caja.
  const paid = await api('POST', `/tickets/${t.ID}/settle`, { paid_cents: total, method });
  state.quote = null;
  await refresh();

  $('#quote-card').innerHTML = `
    <div id="receipt">
      <h2>Recibo de parqueo</h2>
      <p class="muted">${esc(state.site.Name)}</p>
      <dl class="kv">
        <dt>Placa</dt><dd>${esc(paid.PlateNormalized)}</dd>
        <dt>Espacio</dt><dd>${esc(spaceCode(paid.SpaceID))}</dd>
        <dt>Entrada</dt><dd>${esc(fmtTime(paid.EntryAt))}</dd>
        <dt>Salida</dt><dd>${esc(fmtTime(paid.ExitAt))}</dd>
        <dt>Tiempo</dt><dd>${duration(paid.EntryAt, paid.ExitAt)}</dd>
        <dt>Método</dt><dd>${esc(METHODS[method] || method)}</dd>
        <dt>Total</dt><dd>${money(paid.TotalCents)}</dd>
        ${method === 'cash' ? `<dt>Recibido</dt><dd>${money(received)}</dd><dt>Vuelto</dt><dd>${money(received - total)}</dd>` : ''}
      </dl>
      <button class="btn no-print" id="print">Imprimir recibo</button>
    </div>`;
  $('#exit-plate').value = '';
  toast(`Salida de ${paid.PlateNormalized} registrada`);
}

// ---------- Reportes y tarifas ----------

async function loadReports() {
  const date = $('#cash-date').value;
  const from = new Date(Date.now() - 6 * 864e5).toLocaleDateString('en-CA', { timeZone: TZ });
  const to = new Date().toLocaleDateString('en-CA', { timeZone: TZ });
  const [cash, revenue, paid] = await Promise.all([
    api('GET', `/reports/cash-daily?site_id=${state.site.ID}&date=${date}`),
    api('GET', `/reports/revenue?site_id=${state.site.ID}&from=${from}&to=${to}`),
    api('GET', `/tickets?site_id=${state.site.ID}&status=paid&limit=20`),
  ]);

  const methods = Object.entries(cash.ByMethod || {});
  $('#cash-body').innerHTML = `
    <div class="total">${money(cash.TotalCents)}</div>
    <p class="muted">${cash.Count} pagos registrados</p>
    <ul class="breakdown">${methods.map(([m, c]) => `<li><span>${esc(METHODS[m] || m)}</span><span>${money(c)}</span></li>`).join('') || '<li><span class="muted">Sin pagos este día</span><span></span></li>'}</ul>`;

  const days = revenue || [];
  const max = Math.max(1, ...days.map((d) => d.TotalCents));
  $('#revenue-body').innerHTML = days.length ? `<ul class="breakdown">${days.map((d) => `
    <li><span>${esc(d.Date)} · ${d.Count} pagos</span><span>${money(d.TotalCents)}</span></li>
    <li style="border:0;padding:0 0 .4rem"><div class="bar" style="flex:1"><i style="width:${(d.TotalCents / max) * 100}%"></i></div></li>`).join('')}</ul>`
    : '<p class="empty">Sin ingresos en los últimos 7 días.</p>';

  $('#history-body').innerHTML = (paid.data || []).map((t) => `
    <tr><td>${esc(t.PlateNormalized)}</td><td>${esc(fmtTime(t.EntryAt))}</td><td>${t.ExitAt ? esc(fmtTime(t.ExitAt)) : '—'}</td>
    <td>${t.ExitAt ? duration(t.EntryAt, t.ExitAt) : '—'}</td><td class="num">${money(t.TotalCents)}</td><td><span class="badge ok">Pagado</span></td></tr>`).join('')
    || '<tr><td colspan="6" class="empty">Aún no hay salidas.</td></tr>';
}

async function loadRates() {
  const plans = await api('GET', `/rate-plans?site_id=${state.site.ID}`);
  $('#rates-body').innerHTML = (plans || []).filter((p) => p.IsActive).map((p) => `
    <tr><td><strong>${esc(p.Name)}</strong></td><td>${p.GraceMinutes} min</td><td>${p.FractionMinutes} min</td>
    <td class="num">${money(p.CentsPerFraction)}</td><td class="num">${money(p.MinimumCents)}</td><td class="num">${money(p.DailyCapCents)}</td></tr>`).join('')
    || '<tr><td colspan="6" class="empty">No hay tarifas.</td></tr>';
}

// ---------- Navegación y eventos ----------

function openTab(name) {
  document.querySelectorAll('.tabs button').forEach((b) => b.classList.toggle('active', b.dataset.tab === name));
  document.querySelectorAll('.tab').forEach((s) => { s.hidden = s.id !== 'tab-' + name; });
  if (name === 'reportes') loadReports().catch((e) => toast(e.message));
}

function goCharge(plate) {
  openTab('salida');
  $('#exit-plate').value = plate;
  quoteByPlate(plate).catch((e) => showError($('#exit-error'), e.message));
}

// Ejecuta un handler async mostrando el error en el elemento indicado.
const guard = (fn, errEl) => async (e) => {
  e.preventDefault();
  const btn = e.target.querySelector?.('button[type=submit]');
  if (btn) btn.disabled = true;
  try { await fn(e.target); } catch (err) { errEl ? showError(errEl, err.message) : toast(err.message); } finally { if (btn) btn.disabled = false; }
};

$('#login-form').addEventListener('submit', guard((f) => login(f.username.value.trim(), f.password.value), $('#login-error')));
$('#entry-form').addEventListener('submit', guard(registerEntry, $('#entry-error')));
$('#exit-form').addEventListener('submit', guard((f) => quoteByPlate(f.plate.value), $('#exit-error')));
$('#logout').addEventListener('click', logout);
$('#entry-type').addEventListener('change', renderSpaceOptions);
$('#cash-date').addEventListener('change', () => loadReports().catch((e) => toast(e.message)));
document.querySelectorAll('.tabs button').forEach((b) => b.addEventListener('click', () => openTab(b.dataset.tab)));

document.addEventListener('click', (e) => {
  const charge = e.target.closest('[data-charge], .space[data-plate]');
  if (charge) goCharge(charge.dataset.charge || charge.dataset.plate);
  if (e.target.id === 'print') window.print();
});
document.addEventListener('submit', (e) => { if (e.target.id === 'pay-form') guard(settle)(e); });
document.addEventListener('input', (e) => { if (e.target.closest('#pay-form')) updateChange(); });

function tick() {
  $('#clock').textContent = new Date().toLocaleString('es-CR', { timeZone: TZ, weekday: 'long', hour: '2-digit', minute: '2-digit' });
}

// Arranque
$('#cash-date').value = new Date().toLocaleDateString('en-CA', { timeZone: TZ });
tick();
setInterval(tick, 30000);
setInterval(() => { if (state.token && !$('#app-view').hidden) refresh().catch(() => {}); }, 15000);

state.token = storage('get', 'token');
if (state.token) {
  start().catch(() => logout());
} else {
  logout();
}
