/**
 * rtsp.js — RTSP stream display via Go-side MJPEG relay.
 */
import { StartRTSPRelay, StopRTSPRelay } from './bindings.js';

/**
 * Starts the RTSP relay for sourceId and attaches the MJPEG stream to imgEl.
 * @param {HTMLImageElement} imgEl
 * @param {string} sourceId
 * @param {(msg: string) => void} onStatus  called with status text updates
 * @returns {Promise<void>}
 */
export async function playRTSP(imgEl, sourceId, onStatus) {
  onStatus('Connecting…');
  imgEl.src = '';

  let port;
  try {
    port = await StartRTSPRelay(sourceId);
  } catch (err) {
    onStatus(`Failed to start relay: ${err}`);
    return;
  }

  const url = `http://127.0.0.1:${port}/stream`;
  imgEl.src = url;
  imgEl.onload = () => onStatus('');
  imgEl.onerror = () => onStatus('Stream error – check RTSP URL');
}

/**
 * Stops the RTSP relay and clears the image element.
 * @param {HTMLImageElement} imgEl
 * @param {string} sourceId
 */
export async function stopRTSP(imgEl, sourceId) {
  imgEl.src = '';
  imgEl.onload = null;
  imgEl.onerror = null;
  if (sourceId) {
    await StopRTSPRelay(sourceId).catch(() => {});
  }
}
