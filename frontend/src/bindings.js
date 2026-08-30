/**
 * bindings.js — Typed wrappers around the Wails v3 runtime Call() API.
 * Each function maps to an exported method on the Go AppService struct.
 */
import { Call } from '@wailsio/runtime';

// Wails v3 registers services using the Go package path from reflection.
// For types in `package main`, reflect.PkgPath() returns "main", not the module path.
const PKG = 'main.AppService';
// Call is a namespace; ByName(methodName, ...args) is the callable helper.
const invoke = (method, ...args) => Call.ByName(`${PKG}.${method}`, ...args);

/** @returns {Promise<{sources: Array, lastMonitorIndex: number}>} */
export const GetConfig = () => invoke('GetConfig');

/** @param {{ sources: Array, lastMonitorIndex: number }} cfg */
export const SaveConfig = (cfg) => invoke('SaveConfig', cfg);

/** @param {string} sourceId */
export const SetDefaultSource = (sourceId) => invoke('SetDefaultSource', sourceId);

/** @returns {Promise<Array<{index:number,name:string,width:number,height:number,isPrimary:boolean}>>} */
export const GetScreens = () => invoke('GetScreens');

/**
 * Opens a fullscreen projection window on the given monitor.
 * @param {number} screenIndex
 */
export const StartProjection = (screenIndex) => invoke('StartProjection', screenIndex);

/** Closes the projection window. */
export const StopProjection = () => invoke('StopProjection');

/**
 * Starts an RTSP→MJPEG relay for the given source ID.
 * @param {string} sourceId
 * @returns {Promise<number>} localhost port for the MJPEG stream
 */
export const StartRTSPRelay = (sourceId) => invoke('StartRTSPRelay', sourceId);

/**
 * Stops the RTSP relay for the given source ID.
 * @param {string} sourceId
 */
export const StopRTSPRelay = (sourceId) => invoke('StopRTSPRelay', sourceId);

/**
 * Returns the names of all cameras visible to macOS (via system_profiler).
 * Does NOT require camera permission — enumeration is done on the Go side.
 * @returns {Promise<string[]>}
 */
export const GetUSBCameras = () => invoke('GetUSBCameras');
