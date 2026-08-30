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
import { playUSBCamera, stopUSBCamera } from './camera.js';
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

  const videoEl = document.getElementById('usb-video');
  const rtspImg = document.getElementById('rtsp-img');
  const emptyEl = document.getElementById('empty-state');
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

    showStatus('Connecting…');
    videoEl.classList.remove('active');
    rtspImg.classList.remove('active');
    emptyEl.style.display = 'none';

    if (src.type === 'usb') {
      const deviceId = src.id.replace(/^usb:/, '');
      try {
        await playUSBCamera(videoEl, deviceId);
        videoEl.classList.add('active');
        showStatus('');
      } catch (err) {
        showStatus(`Camera error: ${err.message}`);
        activeSource = null;
        emptyEl.style.display = 'flex';
        refreshSidebar();
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
    stopUSBCamera(videoEl);
    videoEl.classList.remove('active');
    await stopRTSP(rtspImg, prev.type === 'rtsp' ? prev.id : null).catch(() => {});
    rtspImg.classList.remove('active');
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

  // ── Status overlay ───────────────────────────────────────────────────────

  let statusTimeout;
  function showStatus(msg) {
    if (!statusEl) return;
    clearTimeout(statusTimeout);
    statusEl.textContent = msg;
    statusEl.classList.toggle('visible', !!msg);
    if (msg) statusTimeout = setTimeout(() => statusEl.classList.remove('visible'), 4000);
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
          <video id="usb-video" class="video-el" autoplay playsinline muted></video>
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
        <button id="settings-close" class="icon-btn" title="Back">←</button>
        <h2>Settings</h2>
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
