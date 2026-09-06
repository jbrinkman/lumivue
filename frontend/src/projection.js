/**
 * projection.js — Projection window mode (/?mode=projection).
 *
 * The projection window receives its active-source information via Wails events
 * from the main window. It renders the same MJPEG feed fullscreen.
 */
import { Events } from '@wailsio/runtime';
import { StopProjection } from './bindings.js';
import { playUSBCamera, stopUSBCamera, getCameraErrorMessage } from './camera.js';
import { playRTSP, stopRTSP } from './rtsp.js';

/**
 * Initialises the projection window.
 * Called by main.js when mode=projection is detected.
 */
export function initProjection() {
  document.getElementById('root').innerHTML = `
    <div class="projection-app">
      <img id="proj-usb-img" alt="" draggable="false" style="display:none" />
      <img id="proj-img" alt="" draggable="false" style="display:none" />
      <div id="proj-empty" style="color:#555;font-size:18px;white-space:pre-line;text-align:center">Waiting for source…</div>
    </div>`;

  const usbImg = document.getElementById('proj-usb-img');
  const rtspImg = document.getElementById('proj-img');
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
      await stopUSBCamera(usbImg, activeSourceId).catch(() => {});
      await stopRTSP(rtspImg, activeSourceId).catch(() => {});
    }
    activeSourceId = src.id;

    usbImg.style.display = 'none';
    rtspImg.style.display = 'none';
    emptyEl.style.display = 'block';
    emptyEl.textContent = 'Waiting for source…';

    if (src.type === 'usb') {
      try {
        await playUSBCamera(usbImg, src.id, src.name, (msg) => {
          if (msg) {
            emptyEl.textContent = msg.replace(/^USB error:\s*/, '');
            emptyEl.style.display = 'block';
            usbImg.style.display = 'none';
          } else {
            emptyEl.style.display = 'none';
            usbImg.style.display = 'block';
          }
        });
      } catch (err) {
        const { title, hint } = getCameraErrorMessage(err);
        emptyEl.textContent = hint ? `${title}\n${hint}` : title;
        emptyEl.style.display = 'block';
        usbImg.style.display = 'none';
      }
    } else if (src.type === 'rtsp') {
      rtspImg.style.display = 'block';
      emptyEl.style.display = 'none';
      await playRTSP(rtspImg, src.id, (msg) => {
        if (msg) {
          emptyEl.textContent = msg;
          emptyEl.style.display = 'block';
          rtspImg.style.display = 'none';
        } else {
          emptyEl.style.display = 'none';
          rtspImg.style.display = 'block';
        }
      });
    }
  });
}
