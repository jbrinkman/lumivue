/**
 * settings.js — Settings panel: USB camera enumeration and RTSP stream config.
 */
import { GetConfig, SaveConfig, GetUSBCameras } from './bindings.js';

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

  body.innerHTML = usbSection + rtspSection;

  // Wire USB rename / remove.
  body.querySelectorAll('[data-action="usb-rename"]').forEach((btn) => {
    btn.addEventListener('click', () => handleUSBRename(btn.dataset.id, cfg, onConfigChange));
  });
  body.querySelectorAll('[data-action="usb-remove"]').forEach((btn) => {
    btn.addEventListener('click', () => handleUSBRemove(btn.dataset.id, cfg, onConfigChange));
  });

  // Wire RTSP edit / remove.
  body.querySelectorAll('[data-action="rtsp-edit"]').forEach((btn) => {
    btn.addEventListener('click', () => handleRTSPEdit(btn.dataset.id, cfg, onConfigChange));
  });
  body.querySelectorAll('[data-action="rtsp-remove"]').forEach((btn) => {
    btn.addEventListener('click', () => handleRTSPRemove(btn.dataset.id, cfg, onConfigChange));
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
}

// ─── USB ──────────────────────────────────────────────────────────────────

// usbDevices is string[] — camera names returned by GetUSBCameras() (Go/system_profiler).
// Cameras are identified by name; the deviceId is resolved at play-time.
function buildUSBSection(usbDevices, savedUSB) {
  const savedNames = new Set(savedUSB.map((s) => s.name));

  const savedRows = savedUSB.map((s) => `
    <div class="source-row">
      <div class="source-row-info">
        <div class="source-row-name">${esc(s.name)}</div>
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

async function handleUSBRename(sourceId, cfg, onConfigChange) {
  const sources = cfg.sources || [];
  const src = sources.find((s) => s.id === sourceId);
  if (!src) return;
  const newName = prompt('Rename camera:', src.name);
  if (!newName || newName === src.name) return;
  const newCfg = { ...cfg, sources: sources.map((s) => s.id === sourceId ? { ...s, name: newName } : s) };
  await SaveConfig(newCfg);
  onConfigChange(newCfg);
  await renderSettings(onConfigChange);
}

async function handleUSBRemove(sourceId, cfg, onConfigChange) {
  if (!confirm('Remove this camera from the list?')) return;
  const newCfg = { ...cfg, sources: (cfg.sources || []).filter((s) => s.id !== sourceId) };
  await SaveConfig(newCfg);
  onConfigChange(newCfg);
  await renderSettings(onConfigChange);
}

// ─── RTSP ─────────────────────────────────────────────────────────────────

function buildRTSPSection(rtspSources) {
  const rows = rtspSources.map((s) => `
    <div class="source-row">
      <div class="source-row-info">
        <div class="source-row-name">${esc(s.name)}</div>
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

async function handleRTSPEdit(sourceId, cfg, onConfigChange) {
  const rtspSources = cfg.sources || [];
  const src = rtspSources.find((s) => s.id === sourceId);
  if (!src) return;
  const newName = prompt('Edit stream name:', src.name);
  if (!newName || newName === src.name) return;
  const newCfg = { ...cfg, sources: rtspSources.map((s) => s.id === sourceId ? { ...s, name: newName } : s) };
  await SaveConfig(newCfg);
  onConfigChange(newCfg);
  await renderSettings(onConfigChange);
}

async function handleRTSPRemove(sourceId, cfg, onConfigChange) {
  if (!confirm('Remove this RTSP stream?')) return;
  const newCfg = { ...cfg, sources: (cfg.sources || []).filter((s) => s.id !== sourceId) };
  await SaveConfig(newCfg);
  onConfigChange(newCfg);
  await renderSettings(onConfigChange);
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
