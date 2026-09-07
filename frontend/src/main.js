/**
 * main.js — Lumivue application entry point.
 *
 * Detects projection vs main mode, then initialises the appropriate UI.
 */
import { Events } from '@wailsio/runtime';
import {
  GetConfig,
  SaveConfig,
  SetDefaultSource,
  GetScreens,
  StartProjection,
  StopProjection,
} from './bindings.js';
import { renderSidebar } from './sidebar.js';
import { renderSettings } from './settings.js';
import { playUSBCamera, stopUSBCamera, getCameraErrorMessage } from './camera.js';
import { playRTSP, stopRTSP } from './rtsp.js';
import { initProjection } from './projection.js';

// ── Mode detection ────────────────────────────────────────────────────────

const params = new URLSearchParams(location.search);
if (params.get('mode') === 'projection') {
  initProjection();
} else {
  initMain();
}

// ── Main window ───────────────────────────────────────────────────────────

async function initMain() {
  renderAppShell();

  const usbImg = document.getElementById('usb-img');
  const rtspImg = document.getElementById('rtsp-img');
  const emptyEl = document.getElementById('empty-state');
  const errorEl = document.getElementById('error-state');
  const errorMsgEl = document.getElementById('error-state-msg');
  const errorHintEl = document.getElementById('error-state-hint');
  const statusEl = document.getElementById('status-overlay');

  let cfg = await GetConfig().catch(() => ({ sources: [], lastMonitorIndex: 0 }));
  let activeSource = null;
  let isProjecting = false;

  // Initial render.
  refreshSidebar();
  await refreshMonitorSelect();

  // ── Settings panel ──────────────────────────────────────────────────────

  document.getElementById('settings-btn').addEventListener('click', openSettings);
  document.getElementById('settings-close').addEventListener('click', closeSettings);

  async function openSettings() {
    document.getElementById('settings-panel').classList.remove('hidden');
    await renderSettings(handleConfigChange);
  }

  function closeSettings() {
    document.getElementById('settings-panel').classList.add('hidden');
  }

  async function handleConfigChange(newCfg) {
    cfg = newCfg;
    refreshSidebar();
    // If active source was removed, stop playback.
    if (activeSource && !cfg.sources.some((s) => s.id === activeSource.id)) {
      await deactivateSource();
    }
  }

  // ── Source activation ───────────────────────────────────────────────────

  async function activateSource(src) {
    if (activeSource?.id === src.id) return;
    await deactivateSource();
    activeSource = src;
    refreshSidebar();

    showError('');
    showStatus('Connecting…');
    usbImg.classList.remove('active');
    rtspImg.classList.remove('active');
    emptyEl.style.display = 'none';

    if (src.type === 'usb') {
      try {
        usbImg.classList.add('active');
        await playUSBCamera(usbImg, src.id, src.displayName || src.name, onUSBCameraStatus);
      } catch (err) {
        usbImg.classList.remove('active');
        const { title, hint } = getCameraErrorMessage(err);
        showStatus('');
        showError({ title, hint });
        activeSource = null;
        refreshSidebar();
        return;
      }
    } else if (src.type === 'rtsp') {
      rtspImg.classList.add('active');
      await playRTSP(rtspImg, src.id, showStatus);
    }

    // Notify projection window of the new source.
    Events.Emit('source:changed', src);
  }

  async function deactivateSource() {
    if (!activeSource) return;
    const prev = activeSource;
    activeSource = null;
    await stopUSBCamera(usbImg, prev.type === 'usb' ? prev.id : null).catch(() => {});
    usbImg.classList.remove('active');
    await stopRTSP(rtspImg, prev.type === 'rtsp' ? prev.id : null).catch(() => {});
    rtspImg.classList.remove('active');
    showError('');
    emptyEl.style.display = 'flex';
    showStatus('');
    refreshSidebar();
  }

  // ── Sidebar ─────────────────────────────────────────────────────────────

  function refreshSidebar() {
    renderSidebar(
      cfg.sources || [],
      activeSource?.id ?? null,
      (src) => activateSource(src),
      async (sourceId) => {
        await SetDefaultSource(sourceId);
        cfg = await GetConfig();
        refreshSidebar();
      },
    );

    // Wire settings link from sidebar footer.
    const addBtn = document.getElementById('add-source-btn');
    if (addBtn) addBtn.addEventListener('click', openSettings);
  }

  // ── Projection ───────────────────────────────────────────────────────────

  async function refreshMonitorSelect() {
    const select = document.getElementById('monitor-select');
    if (!select) return;
    const screens = await GetScreens().catch(() => []);
    select.innerHTML = screens.map((s, i) =>
      `<option value="${i}"${i === cfg.lastMonitorIndex ? ' selected' : ''}>
        ${escapeHtml(`${s.name} ${s.width}×${s.height}${s.isPrimary ? ' (Primary)' : ''}`)}
      </option>`,
    ).join('');
    if (!screens.length) select.innerHTML = '<option value="0">Default monitor</option>';
  }

  document.getElementById('project-btn').addEventListener('click', async () => {
    const btn = document.getElementById('project-btn');
    const statusSpan = document.getElementById('projection-status');

    if (isProjecting) {
      await StopProjection().catch(() => {});
      isProjecting = false;
      btn.textContent = 'Project';
      btn.classList.remove('projecting');
      if (statusSpan) statusSpan.textContent = '';
    } else {
      const select = document.getElementById('monitor-select');
      const screenIndex = parseInt(select?.value ?? '0', 10);
      await StartProjection(screenIndex).catch((err) => {
        alert(`Could not start projection: ${err}`);
      });
      isProjecting = true;
      btn.textContent = 'Stop Projection';
      btn.classList.add('projecting');
      if (statusSpan) statusSpan.textContent = 'Projecting';
      // Send current source to projection window.
      if (activeSource) Events.Emit('source:changed', activeSource);
    }
  });

  // ── Wails backend events ─────────────────────────────────────────────────

  Events.On('rtsp:error', (evt) => {
    const data = evt?.data;
    if (data?.sourceId === activeSource?.id) {
      showStatus(`RTSP error: ${data.error}`);
    }
  });

  Events.On('rtsp:reconnecting', (evt) => {
    const data = evt?.data;
    if (data?.sourceId === activeSource?.id) {
      showStatus('Reconnecting…');
    }
  });

  Events.On('projection:closed', () => {
    isProjecting = false;
    const btn = document.getElementById('project-btn');
    if (btn) { btn.textContent = 'Project'; btn.classList.remove('projecting'); }
    const span = document.getElementById('projection-status');
    if (span) span.textContent = '';
  });

  // ── Status / error display ───────────────────────────────────────────────

  async function onUSBCameraStatus(msg) {
    if (!msg) {
      showStatus('');
      return;
    }
    if (msg.startsWith('USB error:') || msg.startsWith('USB stream error')) {
      const sourceId = msg.startsWith('USB error:') && activeSource?.type === 'usb' ? activeSource.id : null;
      await stopUSBCamera(usbImg, sourceId).catch(() => {});
      usbImg.classList.remove('active');
      activeSource = null;
      showStatus('');
      showError({ title: 'USB camera error', hint: msg.replace(/^USB error:\s*/, '') });
      refreshSidebar();
    } else {
      showStatus(msg);
    }
  }

  let statusTimeout;
  function showStatus(msg) {
    if (!statusEl) return;
    clearTimeout(statusTimeout);
    statusEl.textContent = msg;
    statusEl.classList.toggle('visible', !!msg);
    if (msg) statusTimeout = setTimeout(() => statusEl.classList.remove('visible'), 4000);
  }

  // Persistent, prominent error shown in the main view area.
  // Pass an empty string or falsy value to clear; pass { title, hint } for
  // a categorized camera error with a secondary explanation line.
  function showError(msg) {
    if (!errorEl || !errorMsgEl) return;
    if (!msg) {
      errorEl.style.display = 'none';
      return;
    }
    if (typeof msg === 'string') {
      errorMsgEl.textContent = msg;
      if (errorHintEl) errorHintEl.style.display = 'none';
    } else {
      errorMsgEl.textContent = msg.title;
      if (errorHintEl) {
        errorHintEl.textContent = msg.hint;
        errorHintEl.style.display = 'block';
      }
    }
    errorEl.style.display = 'flex';
    emptyEl.style.display = 'none';
  }
}

// ── App shell HTML ─────────────────────────────────────────────────────────

function renderAppShell() {
  document.getElementById('root').innerHTML = `
    <div class="app">
      <header class="titlebar">
        <span class="titlebar-title">Lumivue</span>
        <button id="settings-btn" class="icon-btn" title="Settings (⌘,)">⚙</button>
      </header>

      <div class="content-row">
        <aside id="sidebar" class="sidebar"></aside>
        <main class="view" id="view">
          <div id="empty-state" class="empty-state" style="display:flex;flex-direction:column;align-items:center;justify-content:center">
            <p>No source selected</p>
            <p class="hint">Select a camera or stream from the sidebar</p>
          </div>
          <div id="error-state" class="error-state" style="display:none">
            <div class="error-state-icon">⚠</div>
            <p id="error-state-msg" class="error-state-msg"></p>
            <p id="error-state-hint" class="error-state-hint" style="display:none"></p>
          </div>
          <img id="usb-img" class="video-el" alt="" draggable="false" />
          <img id="rtsp-img" class="video-el" alt="" draggable="false" />
          <div id="status-overlay" class="status-overlay"></div>
        </main>
      </div>

      <footer class="footer">
        <select id="monitor-select" class="monitor-select" title="Monitor for projection"></select>
        <button id="project-btn" class="project-btn">Project</button>
        <span id="projection-status" class="projection-status"></span>
      </footer>
    </div>

    <div id="settings-panel" class="settings-panel hidden">
      <div class="settings-header">
        <h2>Settings</h2>
        <button id="settings-close" class="icon-btn" title="Close">✕</button>
      </div>
      <div class="settings-body" id="settings-body"></div>
    </div>`;
}

function escapeHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}
