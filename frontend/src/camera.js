/**
 * camera.js — USB camera playback via the Go-side MJPEG relay.
 *
 * Camera enumeration is done on the Go side (GetUSBCameras via system_profiler)
 * so no browser camera permission is needed just to list available devices.
 * Permission is only required when the OS opens the camera in the backend.
 */

import { Events } from '@wailsio/runtime';
import { StartUSBCamera, StopUSBCamera } from './bindings.js';

/**
 * Categorized error produced when a USB camera cannot be played.
 *
 * @property {'permission-denied'|'device-not-found'|'unknown'} category
 * @property {string} cameraName
 * @property {string} originalMessage
 */
export class CameraError extends Error {
  constructor(category, cameraName, originalMessage = '') {
    super(originalMessage);
    this.category = category;
    this.cameraName = cameraName;
    this.originalMessage = originalMessage;
  }
}

/**
 * Returns a user-facing title and hint for a CameraError.
 *
 * @param {CameraError} err
 * @returns {{title: string, hint: string}}
 */
export function getCameraErrorMessage(err) {
  const cameraName = err?.cameraName || '';
  const name = cameraName ? `"${cameraName}"` : 'camera';
  switch (err?.category) {
    case 'permission-denied':
      return {
        title: 'Camera permission was denied.',
        hint: 'Grant camera access to Lumivue in System Settings.',
      };
    case 'device-not-found':
      return {
        title: `Camera ${name} was not found.`,
        hint: 'Make sure it is still connected.',
      };
    case 'unknown':
    default:
      return {
        title: `Could not start camera ${name}.`,
        hint: 'Check the application logs for details.',
      };
  }
}

let activeSourceId = null;
let unsubscribeError = null;

/**
 * Starts the USB camera stream for the given source and attaches it to the
 * provided <img> element as an MJPEG stream.
 *
 * @param {HTMLImageElement} imgEl
 * @param {string} sourceId
 * @param {string} cameraName
 * @param {(msg: string) => void} onStatus  called with status text updates
 * @returns {Promise<void>}
 */
export async function playUSBCamera(imgEl, sourceId, cameraName, onStatus) {
  await stopUSBCamera(imgEl, activeSourceId);

  onStatus('Connecting…');
  imgEl.src = '';

  let port;
  try {
    port = await StartUSBCamera(sourceId);
  } catch (err) {
    const msg = String(err);
    let category = 'unknown';
    if (/permission|denied/i.test(msg)) {
      category = 'permission-denied';
    } else if (/not found|disconnected|no such/i.test(msg)) {
      category = 'device-not-found';
    }
    throw new CameraError(category, cameraName, msg);
  }

  const url = `http://127.0.0.1:${port}/stream`;
  imgEl.src = url;
  imgEl.onload = () => onStatus('');
  imgEl.onerror = () => onStatus('USB stream error – check the application logs');

  unsubscribeError = Events.On('usb:error', (evt) => {
    const data = evt?.data;
    if (data?.sourceId === sourceId) {
      onStatus(`USB error: ${data.error}`);
    }
  });

  activeSourceId = sourceId;
}

/**
 * Stops the active USB camera stream and clears the image element.
 *
 * @param {HTMLImageElement} imgEl
 * @param {string} [sourceId]
 */
export async function stopUSBCamera(imgEl, sourceId) {
  imgEl.src = '';
  imgEl.onload = null;
  imgEl.onerror = null;

  if (unsubscribeError) {
    unsubscribeError();
    unsubscribeError = null;
  }

  if (sourceId) {
    await StopUSBCamera(sourceId).catch(() => {});
  }

  activeSourceId = null;
}
