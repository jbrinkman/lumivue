//go:build darwin
// +build darwin

package main

import (
	"testing"

	"github.com/asticode/go-astiav"
)

func TestParseAVFoundationDeviceList(t *testing.T) {
	logs := []string{
		"[AVFoundation indev @ 0x0] AVFoundation video devices:",
		"[AVFoundation indev @ 0x0] [0] FaceTime HD Camera",
		"[AVFoundation indev @ 0x0] [1] USB Camera",
		"[AVFoundation indev @ 0x0] [2] Capture screen 0",
		"[AVFoundation indev @ 0x0] AVFoundation audio devices:",
	}

	devs := parseAVFoundationDeviceList(logs)
	if len(devs) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devs))
	}
	if devs[0].Index != 0 || devs[0].Name != "FaceTime HD Camera" {
		t.Errorf("unexpected first device: %+v", devs[0])
	}
	if devs[1].Index != 1 || devs[1].Name != "USB Camera" {
		t.Errorf("unexpected second device: %+v", devs[1])
	}
}

func TestAvfDeviceLister_ListDevices(t *testing.T) {
	l := &avfDeviceLister{}
	devs, err := l.listDevices()
	if err != nil {
		// AVFoundation device listing may fail in sandboxed or headless
		// environments; the test only verifies the lister can be invoked.
		t.Logf("listDevices returned error: %v", err)
	}
	// Result may be nil or empty if no cameras are available.
	_ = devs
}

func TestOpenAVFoundationDevice_InvalidIndex(t *testing.T) {
	astiav.RegisterAllDevices()

	_, _, err := openAVFoundationDevice(999)
	if err == nil {
		t.Fatal("expected error for non-existent AVFoundation device index")
	}
}
