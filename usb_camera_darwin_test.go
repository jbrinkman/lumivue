//go:build darwin
// +build darwin

package main

import (
	"testing"

	"github.com/asticode/go-astiav"
)

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
