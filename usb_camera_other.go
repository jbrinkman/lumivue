//go:build !darwin
// +build !darwin

package main

import (
	"fmt"

	"github.com/asticode/go-astiav"
)

type stubDeviceLister struct{}

func (l *stubDeviceLister) listDevices() ([]usbCameraDevice, error) {
	return nil, fmt.Errorf("USB camera capture is only supported on macOS")
}

func init() {
	defaultDeviceLister = &stubDeviceLister{}
	openUSBCamera = stubOpenUSBCamera
}

// stubOpenUSBCamera is a non-darwin stub that always returns an error.
func stubOpenUSBCamera(deviceIndex int) (*astiav.FormatContext, *astiav.Stream, error) {
	return nil, nil, fmt.Errorf("USB camera capture is only supported on macOS")
}
