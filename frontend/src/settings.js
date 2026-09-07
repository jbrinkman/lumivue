/**
 * settings.js — Settings panel: USB camera enumeration and RTSP stream config.
 */
import { GetConfig, SaveConfig, GetUSBCameras, GetLogPath, RevealLogsInFinder } from './bindings.js';

/**
 * Renders the full settings panel content into #settings-body and wires interactions.
 * Calls onConfigChange(newConfig) whenever config is saved.
 * @param {(cfg: object) => void} onConfigChange
 */
export async function renderSettings(onConfigChange) {
  const body = document.getElementById('settings-body');
  if (!body) return;

  body.innerHTML = '<p style="color:var(--text-muted)">Loading…</p>';

  let cfg, usbDevices;
  try {
    // GetUSBCameras() enumerates via system_profiler on the Go side —
    // no browser camera permission is required to list cameras.
    [cfg, usbDevices] = await Promise.all([GetConfig(), GetUSBCameras()]);
  } catch (err) {
    const msg = err?.message || (typeof err === 'string' ? err : JSON.stringify(err)) || 'backend unavailable';
    body.innerHTML = `<p style="color:var(--danger);padding:8px">Failed to load settings: ${msg}</p>`;
    return;
  }

  // Build USB section: devices detected by the browser, cross-referenced with saved sources.
  const usbSources = (cfg.sources || []).filter((s) => s.type === 'usb');
  const usbSection = buildUSBSection(usbDevices, usbSources);

  // Build RTSP section.
  const rtspSources = (cfg.sources || []).filter((s) => s.type === 'rtsp');
  const rtspSection = buildRTSPSection(rtspSources);

  // Build Logs section.
  let logPath = '';
  try { logPath = await GetLogPath(); } catch (_) {}
  const logsSection = buildLogsSection(logPath);

  body.innerHTML = usbSection + rtspSection + logsSection;

  // Wire USB rename / remove.
  body.querySelectorAll('[data-action="usb-rename"]').forEach((btn) => {
    btn.addEventListener('click', () => startInlineRename(btn, cfg, onConfigChange));
  });
  body.querySelectorAll('[data-action="usb-remove"]').forEach((btn) => {
    btn.addEventListener('click', () => handleUSBRemove(btn, cfg, onConfigChange));
  });

  // Wire RTSP edit / remove.
  body.querySelectorAll('[data-action="rtsp-edit"]').forEach((btn) => {
    btn.addEventListener('click', () => startInlineRename(btn, cfg, onConfigChange));
  });
  body.querySelectorAll('[data-action="rtsp-remove"]').forEach((btn) => {
    btn.addEventListener('click', () => handleRTSPRemove(btn, cfg, onConfigChange));
  });

  // Wire RTSP add form.
  const showAddBtn = body.querySelector('#rtsp-show-add');
  const addForm = body.querySelector('#rtsp-add-form');
  if (showAddBtn && addForm) {
    showAddBtn.addEventListener('click', () => {
      addForm.classList.toggle('hidden');
      showAddBtn.textContent = addForm.classList.contains('hidden') ? '+ Add Stream' : '− Cancel';
    });
  }

  const submitBtn = body.querySelector('#rtsp-submit');
  if (submitBtn) {
    submitBtn.addEventListener('click', () => handleRTSPAdd(cfg, onConfigChange));
  }

  // Wire USB "Add detected camera" buttons.
  body.querySelectorAll('[data-action="usb-add"]').forEach((btn) => {
    btn.addEventListener('click', () => handleUSBAdd(btn.dataset.label, cfg, onConfigChange));
  });

  // Wire Logs section.
  const revealBtn = body.querySelector('#logs-reveal-btn');
  if (revealBtn) {
    revealBtn.addEventListener('click', () => RevealLogsInFinder().catch(() => {}));
  }
}

// ─── USB ──────────────────────────────────────────────────────────────────

// usbDevices is string[] — camera names returned by GetUSBCameras() (Go/system_profiler).
// Cameras are identified by name; the deviceId is resolved at play-time.
function buildUSBSection(usbDevices, savedUSB) {
  const savedNames = new Set(savedUSB.map((s) => s.name));

  const savedRows = savedUSB.map((s) => `
    <div class="source-row">
      <div class="source-row-info">
        <div class="source-row-name">${esc(s.displayName || s.name)}</div>
      </div>
      <button class="text-btn" data-action="usb-rename" data-id="${escAttr(s.id)}">Rename</button>
      <button class="text-btn danger" data-action="usb-remove" data-id="${escAttr(s.id)}">Remove</button>
    </div>`).join('');

  const detectedRows = usbDevices
    .filter((name) => !savedNames.has(name))
    .map((name) => `
      <div class="source-row">
        <div class="source-row-info">
          <div class="source-row-name">${esc(name)}</div>
        </div>
        <button class="text-btn"
            data-action="usb-add"
            data-label="${escAttr(name)}">Add</button>
      </div>`).join('');

  const noneMsg = !savedRows && !detectedRows
    ? '<p style="color:var(--text-muted);font-size:13px">No cameras detected. Connect a USB camera and reopen settings.</p>'
    : '';

  return `
    <div class="settings-section">
      <div class="settings-section-title">USB Cameras</div>
      ${savedRows}${detectedRows}${noneMsg}
    </div>`;
}

// Cameras are identified by name (resolved to a deviceId at play-time via enumerateDevices).
async function handleUSBAdd(name, cfg, onConfigChange) {
  const id = `usb:${name}`;
  const sources = cfg.sources || [];
  if (sources.some((s) => s.id === id)) return;
  const newCfg = { ...cfg, sources: [...sources, { id, type: 'usb', name, isDefault: false }] };
  await SaveConfig(newCfg);
  onConfigChange(newCfg);
  await renderSettings(onConfigChange);
}

// Transforms the source row into an inline edit form.
// WKWebView (Wails) does not reliably support window.prompt(), so we edit in-place.
function startInlineRename(btn, cfg, onConfigChange) {
  const sourceId = btn.dataset.id;
  const sources = cfg.sources || [];
  const src = sources.find((s) => s.id === sourceId);
  if (!src) return;

  const row = btn.closest('.source-row');
  const nameDiv = row.querySelector('.source-row-name');

  // Replace static name with an editable input.
  const originalName = src.displayName || src.name;
  nameDiv.innerHTML =
    `<input class="inline-rename-input" type="text" value="${escAttr(originalName)}" />`;
  const input = nameDiv.querySelector('input');
  input.focus();
  input.select();

  // Swap the Rename button for Save, insert Cancel before it.
  btn.textContent = 'Save';
  const cancelBtn = document.createElement('button');
  cancelBtn.className = 'text-btn';
  cancelBtn.textContent = 'Cancel';
  row.insertBefore(cancelBtn, btn);

  cancelBtn.addEventListener('click', () => renderSettings(onConfigChange));
  btn.addEventListener('click', () => commitRename(sourceId, input.value.trim(), cfg, onConfigChange));
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') commitRename(sourceId, input.value.trim(), cfg, onConfigChange);
    if (e.key === 'Escape') renderSettings(onConfigChange);
  });
}

async function commitRename(sourceId, newName, cfg, onConfigChange) {
  const sources = cfg.sources || [];
  if (!newName) return;
  const src = sources.find((s) => s.id === sourceId);
  if (!src || newName === (src.displayName || src.name)) { await renderSettings(onConfigChange); return; }
  const newCfg = { ...cfg, sources: sources.map((s) => s.id === sourceId ? { ...s, displayName: newName } : s) };
  await SaveConfig(newCfg);
  onConfigChange(newCfg);
  await renderSettings(onConfigChange);
}

function showRemoveConfirm(row, source, cfg, onConfigChange) {
  const label = source.displayName || source.name || source.url || 'this source';
  row.innerHTML = `
    <div class="source-row-info">
      <div class="source-row-name remove-confirm-text">Remove ${esc(label)}?</div>
    </div>
    <button class="text-btn danger confirm-btn">Confirm</button>
    <button class="text-btn cancel-btn">Cancel</button>
  `;

  const confirmBtn = row.querySelector('.confirm-btn');
  const cancelBtn = row.querySelector('.cancel-btn');
  let confirming = false;

  confirmBtn.addEventListener('click', async () => {
    if (confirming) return;
    confirming = true;
    confirmBtn.disabled = true;
    confirmBtn.style.opacity = '0.6';
    try {
      const newCfg = { ...cfg, sources: (cfg.sources || []).filter((s) => s.id !== source.id) };
      await SaveConfig(newCfg);
      onConfigChange(newCfg);
      await renderSettings(onConfigChange);
    } catch (err) {
      confirming = false;
      confirmBtn.disabled = false;
      confirmBtn.style.opacity = '';
    }
  });

  cancelBtn.addEventListener('click', async () => {
    await renderSettings(onConfigChange);
  });
}

async function handleUSBRemove(btn, cfg, onConfigChange) {
  const sourceId = btn.dataset.id;
  const source = (cfg.sources || []).find((s) => s.id === sourceId);
  if (!source) return;
  const row = btn.closest('.source-row');
  if (!row) return;
  showRemoveConfirm(row, source, cfg, onConfigChange);
}

// ─── RTSP ─────────────────────────────────────────────────────────────────

function buildRTSPSection(rtspSources) {
  const rows = rtspSources.map((s) => `
    <div class="source-row">
      <div class="source-row-info">
        <div class="source-row-name">${esc(s.displayName || s.name)}</div>
        <div class="source-row-url">${esc(s.url)}</div>
      </div>
      <button class="text-btn" data-action="rtsp-edit" data-id="${escAttr(s.id)}">Edit</button>
      <button class="text-btn danger" data-action="rtsp-remove" data-id="${escAttr(s.id)}">Remove</button>
    </div>`).join('');

  return `
    <div class="settings-section">
      <div class="settings-section-title">RTSP Streams</div>
      ${rows}
      <button class="text-btn" id="rtsp-show-add" style="margin-top:8px">+ Add Stream</button>
      <div class="add-form hidden" id="rtsp-add-form">
        <div class="form-field">
          <label>Name</label>
          <input id="rtsp-name" type="text" placeholder="Back Camera" />
        </div>
        <div class="form-field">
          <label>RTSP URL</label>
          <input id="rtsp-url" type="text" placeholder="rtsp://192.168.1.100:554/stream" />
          <div class="error-msg hidden" id="rtsp-url-error"></div>
        </div>
        <div class="form-actions">
          <button class="text-btn" id="rtsp-submit">Add Stream</button>
        </div>
      </div>
    </div>`;
}

async function handleRTSPAdd(cfg, onConfigChange) {
  const nameInput = document.getElementById('rtsp-name');
  const urlInput = document.getElementById('rtsp-url');
  const errEl = document.getElementById('rtsp-url-error');

  const name = (nameInput?.value || '').trim();
  const url = (urlInput?.value || '').trim();

  if (errEl) errEl.classList.add('hidden');

  if (!url.startsWith('rtsp://')) {
    if (errEl) {
      errEl.textContent = 'URL must start with rtsp://';
      errEl.classList.remove('hidden');
    }
    return;
  }
  if (!name) {
    nameInput?.focus();
    return;
  }

  const id = `rtsp:${url}`;
  const sources = cfg.sources || [];
  if (sources.some((s) => s.id === id)) {
    if (errEl) {
      errEl.textContent = 'A stream with this URL already exists.';
      errEl.classList.remove('hidden');
    }
    return;
  }

  const newCfg = {
    ...cfg,
    sources: [...sources, { id, type: 'rtsp', name, url, isDefault: false }],
  };
  await SaveConfig(newCfg);
  onConfigChange(newCfg);
  await renderSettings(onConfigChange);
}

async function handleRTSPRemove(btn, cfg, onConfigChange) {
  const sourceId = btn.dataset.id;
  const source = (cfg.sources || []).find((s) => s.id === sourceId);
  if (!source) return;
  const row = btn.closest('.source-row');
  if (!row) return;
  showRemoveConfirm(row, source, cfg, onConfigChange);
}

// ─── Logs ─────────────────────────────────────────────────────────────────

function buildLogsSection(logPath) {
  const pathHtml = logPath
    ? `<div class="source-row-url" style="font-size:11px;color:var(--text-muted);margin-bottom:8px">${esc(logPath)}</div>`
    : '';
  return `
    <div class="settings-section">
      <div class="settings-section-title">Logs</div>
      ${pathHtml}
      <button class="text-btn" id="logs-reveal-btn">Open Log Folder in Finder</button>
    </div>`;
}

// ─── Helpers ──────────────────────────────────────────────────────────────

function esc(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function escAttr(str) {
  return String(str).replace(/"/g, '&quot;');
}
