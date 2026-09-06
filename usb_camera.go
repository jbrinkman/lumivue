package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/asticode/go-astiav"
)

// usbCameraDevice describes one video capture device that AVFoundation can see.
type usbCameraDevice struct {
	Index int
	Name  string
	UID   string
}

// deviceLister abstracts the operation of enumerating available USB/video devices.
type deviceLister interface {
	listDevices() ([]usbCameraDevice, error)
}

// defaultDeviceLister is injected by platform-specific files.
var defaultDeviceLister deviceLister

// openUSBCamera is injected by platform-specific files. It opens the device at
// the given index and returns the format context plus the best video stream.
var openUSBCamera func(deviceIndex int) (*astiav.FormatContext, *astiav.Stream, error)

// normalizeName lowercases a camera name and collapses all whitespace to a single
// space so matching is tolerant of differences in spacing and casing.
func normalizeName(name string) string {
	var b strings.Builder
	var lastSpace bool
	for _, r := range strings.ToLower(name) {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

// resolveUSBCamera matches a configured camera name against the list of devices
// returned by AVFoundation. It first tries an exact normalized match, then a
// substring match in either direction.
func resolveUSBCamera(cameraName string, devices []usbCameraDevice) (int, error) {
	if cameraName == "" {
		return -1, fmt.Errorf("camera name is empty")
	}
	if len(devices) == 0 {
		return -1, fmt.Errorf("no USB cameras detected")
	}

	normalized := normalizeName(cameraName)
	for _, d := range devices {
		if normalizeName(d.Name) == normalized {
			return d.Index, nil
		}
	}

	for _, d := range devices {
		dn := normalizeName(d.Name)
		if strings.Contains(dn, normalized) || strings.Contains(normalized, dn) {
			return d.Index, nil
		}
	}

	return -1, fmt.Errorf("camera %q not found among %d device(s)", cameraName, len(devices))
}
