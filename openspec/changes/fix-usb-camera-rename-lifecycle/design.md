## Context

See `proposal.md` for the user-visible symptoms. The logs show two distinct problems:

1. A renamed USB camera (`"Bella Camera"`) could not be started because `StartUSBCamera` passes `src.Name` to `resolveUSBCamera`, and `resolveUSBCamera` matches against the system device list. The rename changed the only name we stored, so the match failed.
2. After `StartUSBCamera` failed, the process crashed in `USBCameraRelay.captureLoop` with a nil `CodecContext` in the defer. The old relay was stopped while its `captureLoop` was still active; `Stop` freed `decoderCtx` and set it to nil, and when `captureLoop` finally exited it called `r.decoderCtx.Free()` on the nil pointer.
3. The same crash is reproducible by projecting a USB camera to full screen, because `StartUSBCamera` is invoked again before the previous relay has fully shut down. The existing relay is stopped, deleted, and a new relay is created, but the first relay's `captureLoop` is still running and can still call `r.decoderCtx.Free()` after `Stop` has cleared the field.

## Goals / Non-Goals

**Goals:**
- Let users rename USB cameras to custom labels without breaking device resolution.
- Make the `USBCameraRelay` start/stop lifecycle race-free and crash-free.
- Keep the existing source list behavior and config file backward-compatible.

**Non-Goals:**
- Adding a separate device-UID field beyond the device name; the system still identifies USB cameras by the system device name.
- Rewriting the RTSP relay lifecycle; only the USB camera relay is in scope.
- Changing how camera permissions are requested.

## Decisions

- **Add a `DisplayName` field to `Source`.** `Name` continues to hold the system device name for USB sources and the user-supplied name for RTSP sources. `DisplayName` is an optional user override shown in the UI. This is the smallest schema change that preserves backward compatibility (`DisplayName` defaults to empty and the UI falls back to `Name`).
- **USB rename only edits `DisplayName`.** The settings panel's inline rename for USB sources will write `DisplayName` and leave `Name` and `ID` unchanged. RTSP rename can continue to edit `Name` (its identifier is the URL), or also write `DisplayName` for consistency; either is acceptable because RTSP matching uses `URL`.
- **UI rendering uses `DisplayName || Name`.** Anywhere the source label appears (sidebar, settings rows, status messages, projection empty state) the code will prefer `DisplayName` and fall back to `Name`.
- **`StartUSBCamera` resolves by `Name`, not `DisplayName`.** It passes `src.Name` to the relay; `DisplayName` is only used for logs and frontend-facing messages.
- **`USBCameraRelay` owns its own goroutine lifetime.** Add a `sync.WaitGroup` to the relay. `capture` increments it and `captureLoop` defers `Done()`. `Stop` cancels the context, shuts down the HTTP server, and then `Wait()`s for the capture goroutine before freeing `swsCtx`.
- **Only `captureLoop` frees `decoderCtx`.** `decoderCtx` is allocated inside `captureLoop`, so `captureLoop`'s defer frees it. `Stop` does not touch `decoderCtx`; it only frees `swsCtx` after the loop has exited. This removes the double-free / nil-pointer race.
- **Nil-guard all `Free()` calls in defers.** `captureLoop` and `Stop` will check `if r.decoderCtx != nil` / `if r.swsCtx != nil` before calling `Free()` so that unexpected early exits or repeated stops do not panic.
- **`Start` returns an error only for pre-capture failures.** `listDevices` and `resolveUSBCamera` failures still return before the capture goroutine is launched. `openDevice` failures are reported asynchronously via `usb:error` events, but `Start` no longer risks leaking a half-initialized relay because the goroutine and context are fully owned by the relay and cleaned up by `Stop`.
- **Do not start a new relay until the old one has exited.** `AppService.StartUSBCamera` already stops the existing relay before creating a new one, but it does not wait for the old `captureLoop` to finish. The fix is to make `USBCameraRelay.Stop()` block until `captureLoop` exits (via the WaitGroup) and to hold the `AppService` mutex until `Stop` returns, so a second `StartUSBCamera` cannot overwrite the map entry while the old relay is still running.

## Risks / Trade-offs

- [Risk] Adding `DisplayName` to `Source` changes the persisted JSON schema. → Mitigation: the field is optional; `omitempty` keeps existing config files unchanged, and the UI always falls back to `Name`.
- [Risk] Waiting for `captureLoop` to exit in `Stop` may briefly block the UI thread if `ReadFrame` is stuck. → Mitigation: `Stop` first cancels the context and shuts down the HTTP server, then waits with a bounded context (the existing 3-second `server.Shutdown` timeout) and does not wait indefinitely.
- [Risk] `captureLoop` may be blocked in `ReadFrame` and not observe `r.ctx.Done()` quickly. → Mitigation: the existing top-of-loop context check still helps, and the `EAGAIN` retry path keeps the loop responsive. If `ReadFrame` is truly stuck, the `server.Shutdown` timeout will expire and `Stop` will return, but `captureLoop` may continue briefly; the `running` wait group prevents `Stop` from returning resources too early.
- [Risk] `StartUSBCamera` may be called twice in rapid succession (e.g., full-screen projection). → Mitigation: `AppService` holds its mutex while stopping the old relay; `Stop` blocks until `captureLoop` exits, and only then does `StartUSBCamera` unlock and start the new relay.
- [Risk] `displayName` rendering changes could affect existing RTSP sources that relied on `Name` being displayed. → Mitigation: the fallback `DisplayName || Name` preserves the old behavior when `DisplayName` is empty.

## Migration Plan

- No deployment or external migration needed. Existing `config.json` files without `displayName` load cleanly and behave as before. When a user renames a USB source for the first time, the config will gain the `displayName` field.

## Open Questions

- None. The user's answers confirmed that renaming should be display-name-only and that the crash and rename resolution should be handled in one combined change.
