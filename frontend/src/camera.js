/**
 * camera.js — USB camera enumeration and video playback via MediaDevices API.
 */

/**
 * Enumerates available video input (USB camera) devices.
 * Requests temporary permission if needed to get device labels.
 * @returns {Promise<MediaDeviceInfo[]>}
 */
export async function enumerateUSBCameras() {
  // Some browsers only return labels after permission is granted.
  // We ask for a temporary stream and immediately stop it.
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: false });
    stream.getTracks().forEach((t) => t.stop());
  } catch (_) {
    // Permission denied or no camera — continue with empty labels.
  }
  const devices = await navigator.mediaDevices.enumerateDevices();
  return devices.filter((d) => d.kind === 'videoinput');
}

/**
 * Starts the USB camera stream for the given deviceId and attaches it to
 * the provided <video> element.
 * @param {HTMLVideoElement} videoEl
 * @param {string} deviceId
 * @returns {Promise<MediaStream>}
 */
export async function playUSBCamera(videoEl, deviceId) {
  // Stop any current stream first.
  stopUSBCamera(videoEl);

  const stream = await navigator.mediaDevices.getUserMedia({
    video: { deviceId: { exact: deviceId } },
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
