/**
 * camera.js — USB camera playback via MediaDevices API.
 *
 * Camera enumeration is done on the Go side (GetUSBCameras via system_profiler)
 * so no browser camera permission is needed just to list available devices.
 * Permission is only required here, when the user actually plays a camera.
 */

/**
 * Categorized error produced when a USB camera cannot be played.
 *
 * @property {'api-unavailable'|'permission-denied'|'device-not-found'|'unknown'} category
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
    case 'api-unavailable':
      return {
        title: 'Camera playback is not available in this view.',
        hint: 'A native implementation is required to enable USB camera access.',
      };
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

/**
 * Starts the USB camera stream for the device whose label matches `cameraName`
 * and attaches it to the provided <video> element.
 *
 * Resolution order:
 *  1. Call getUserMedia({ video: true }) to trigger the macOS permission dialog
 *     (if permission has not been granted yet) and then stop the temporary stream.
 *  2. Enumerate devices to find the deviceId whose label matches `cameraName`.
 *  3. Open the camera with that specific deviceId.
 *
 * @param {HTMLVideoElement} videoEl
 * @param {string} cameraName  — the camera's OS-level name (from GetUSBCameras)
 * @returns {Promise<MediaStream>}
 */
export async function playUSBCamera(videoEl, cameraName) {
  stopUSBCamera(videoEl);

  if (!navigator.mediaDevices) {
    throw new CameraError(
      'api-unavailable',
      cameraName,
      'Camera access is not available in this context.',
    );
  }

  // Step 1: ensure camera permission (triggers macOS dialog on first use).
  let tmpStream;
  try {
    tmpStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: false });
    tmpStream.getTracks().forEach((t) => t.stop());
  } catch (err) {
    const isPermission =
      err.name === 'NotAllowedError' ||
      err.name === 'PermissionDeniedError' ||
      /permission/i.test(err.message || '');
    throw new CameraError(
      isPermission ? 'permission-denied' : 'unknown',
      cameraName,
      `Camera permission denied: ${err.message}`,
    );
  }

  // Step 2: find the deviceId for the requested camera by label.
  let devices;
  try {
    devices = await navigator.mediaDevices.enumerateDevices();
  } catch (err) {
    throw new CameraError(
      'unknown',
      cameraName,
      `Failed to enumerate cameras: ${err.message}`,
    );
  }

  const device = devices.find((d) => d.kind === 'videoinput' && d.label === cameraName);
  if (!device) {
    throw new CameraError(
      'device-not-found',
      cameraName,
      `Camera "${cameraName}" was not found. It may have been disconnected or its name changed.`,
    );
  }

  // Step 3: open the specific camera.
  try {
    const stream = await navigator.mediaDevices.getUserMedia({
      video: { deviceId: { exact: device.deviceId } },
      audio: false,
    });
    videoEl.srcObject = stream;
    await videoEl.play().catch(() => {});
    return stream;
  } catch (err) {
    const isPermission =
      err.name === 'NotAllowedError' ||
      err.name === 'PermissionDeniedError' ||
      /permission/i.test(err.message || '');
    throw new CameraError(
      isPermission ? 'permission-denied' : 'unknown',
      cameraName,
      `Failed to open camera: ${err.message}`,
    );
  }
}

/**
 * Stops the active USB camera stream attached to the given <video> element.
 * @param {HTMLVideoElement} videoEl
 */
export function stopUSBCamera(videoEl) {
  if (videoEl.srcObject) {
    videoEl.srcObject.getTracks().forEach((t) => t.stop());
    videoEl.srcObject = null;
  }
}
