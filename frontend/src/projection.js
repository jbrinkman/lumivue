/**
 * projection.js — Projection window mode (/?mode=projection).
 *
 * The projection window receives its active-source information via Wails events
 * from the main window. It renders the same video/MJPEG feed fullscreen.
 */
import { Events } from '@wailsio/runtime';
import { StopProjection } from './bindings.js';
import { playUSBCamera, stopUSBCamera } from './camera.js';
import { playRTSP, stopRTSP } from './rtsp.js';

/**
 * Initialises the projection window.
 * Called by main.js when mode=projection is detected.
 */
export function initProjection() {
  document.getElementById('root').innerHTML = `
    <div class="projection-app">
      <video id="proj-video" autoplay playsinline muted style="display:none"></video>
      <img id="proj-img" alt="" draggable="false" style="display:none" />
      <div id="proj-empty" style="color:#555;font-size:18px">Waiting for source…</div>
    </div>`;

  const videoEl = document.getElementById('proj-video');
  const imgEl = document.getElementById('proj-img');
  const emptyEl = document.getElementById('proj-empty');

  let activeSourceId = null;

  // ESC key exits projection (triggers Stop from main window via event).
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      StopProjection().catch(() => {});
    }
  });

  Events.On('source:changed', async (evt) => {
    const src = evt?.data;
    if (!src) return;

    // Stop previous.
    if (activeSourceId) {
      stopUSBCamera(videoEl);
      await stopRTSP(imgEl, activeSourceId).catch(() => {});
    }
    activeSourceId = src.id;

    videoEl.style.display = 'none';
    imgEl.style.display = 'none';
    emptyEl.style.display = 'block';

    if (src.type === 'usb') {
      // deviceId is encoded as usb:<deviceId>
      const deviceId = src.id.replace(/^usb:/, '');
      try {
        await playUSBCamera(videoEl, deviceId);
        videoEl.style.display = 'block';
        emptyEl.style.display = 'none';
      } catch (err) {
        emptyEl.textContent = `Camera error: ${err.message}`;
        emptyEl.style.display = 'block';
      }
    } else if (src.type === 'rtsp') {
      imgEl.style.display = 'block';
      emptyEl.style.display = 'none';
      await playRTSP(imgEl, src.id, (msg) => {
        if (msg) {
          emptyEl.textContent = msg;
          emptyEl.style.display = 'block';
          imgEl.style.display = 'none';
        } else {
          emptyEl.style.display = 'none';
          imgEl.style.display = 'block';
        }
      });
    }
  });
}
