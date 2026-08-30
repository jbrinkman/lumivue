/**
 * camera.js — USB camera playback via MediaDevices API.
 *
 * Camera enumeration is done on the Go side (GetUSBCameras via system_profiler)
 * so no browser camera permission is needed just to list available devices.
 * Permission is only required here, when the user actually plays a camera.
 */

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
    throw new Error('Camera access is not available in this context.');
  }

  // Step 1: ensure camera permission (triggers macOS dialog on first use).
  try {
    const tmp = await navigator.mediaDevices.getUserMedia({ video: true, audio: false });
    tmp.getTracks().forEach((t) => t.stop());
  } catch (err) {
    throw new Error(`Camera permission denied: ${err.message}`);
  }

  // Step 2: find the deviceId for the requested camera by label.
  const devices = await navigator.mediaDevices.enumerateDevices();
  const device = devices.find((d) => d.kind === 'videoinput' && d.label === cameraName);
  if (!device) {
    throw new Error(
      `Camera "${cameraName}" was not found. It may have been disconnected or its name changed.`
    );
  }

  // Step 3: open the specific camera.
  const stream = await navigator.mediaDevices.getUserMedia({
    video: { deviceId: { exact: device.deviceId } },
    audio: false,
  });
  videoEl.srcObject = stream;
  await videoEl.play().catch(() => {});
  return stream;
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
