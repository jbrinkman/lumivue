//go:build darwin
// +build darwin

package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/asticode/go-astiav"
)

func init() {
	defaultDeviceLister = &avfDeviceLister{}
	openUSBCamera = openAVFoundationDevice
}

type avfDeviceLister struct{}

// openAVFoundationDevice opens the AVFoundation device at the given index and
// returns the format context plus the best video stream.
func openAVFoundationDevice(deviceIndex int) (*astiav.FormatContext, *astiav.Stream, error) {
	astiav.RegisterAllDevices()

	inputFormat := astiav.FindInputFormat("avfoundation")
	if inputFormat == nil {
		return nil, nil, fmt.Errorf("avfoundation input format not available")
	}

	formatCtx := astiav.AllocFormatContext()
	if formatCtx == nil {
		return nil, nil, fmt.Errorf("alloc format context")
	}

	dict := astiav.NewDictionary()
	defer dict.Free()
	_ = dict.Set("framerate", "30", astiav.DictionaryFlags(0))
	_ = dict.Set("video_size", "640x480", astiav.DictionaryFlags(0))

	url := fmt.Sprintf("%d:", deviceIndex)
	if err := formatCtx.OpenInput(url, inputFormat, dict); err != nil {
		formatCtx.Free()
		return nil, nil, fmt.Errorf("open avfoundation device %d: %w", deviceIndex, err)
	}

	if err := formatCtx.FindStreamInfo(nil); err != nil {
		formatCtx.CloseInput()
		formatCtx.Free()
		return nil, nil, fmt.Errorf("find stream info: %w", err)
	}

	stream, _, err := formatCtx.FindBestStream(astiav.MediaTypeVideo, -1, -1)
	if err != nil {
		formatCtx.CloseInput()
		formatCtx.Free()
		return nil, nil, fmt.Errorf("find best video stream: %w", err)
	}

	return formatCtx, stream, nil
}

// listDevices uses the avfoundation input format's list_devices option. The
// device list is emitted through the FFmpeg log callback, so we temporarily
// install a callback, call OpenInput with list_devices=1, and parse the
// resulting log lines. avdevice_list_devices is not implemented for avfoundation.
func (l *avfDeviceLister) listDevices() ([]usbCameraDevice, error) {
	astiav.RegisterAllDevices()

	inputFormat := astiav.FindInputFormat("avfoundation")
	if inputFormat == nil {
		return nil, fmt.Errorf("avfoundation input format not available")
	}

	var logs []string
	var mu sync.Mutex
	astiav.SetLogCallback(func(_ astiav.Classer, _ astiav.LogLevel, _, msg string) {
		mu.Lock()
		logs = append(logs, msg)
		mu.Unlock()
	})
	defer astiav.ResetLogCallback()

	formatCtx := astiav.AllocFormatContext()
	if formatCtx == nil {
		return nil, fmt.Errorf("alloc format context")
	}
	defer formatCtx.Free()

	dict := astiav.NewDictionary()
	defer dict.Free()
	_ = dict.Set("list_devices", "1", astiav.DictionaryFlags(0))

	// list_devices always fails after printing the device list; ignore the error.
	_ = formatCtx.OpenInput("", inputFormat, dict)

	out := parseAVFoundationDeviceList(logs)
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// parseAVFoundationDeviceList parses the log output produced by avfoundation's
// list_devices option into usbCameraDevice entries.
func parseAVFoundationDeviceList(logs []string) []usbCameraDevice {
	re := regexp.MustCompile(`\[(\d+)\]\s+(.+)`)
	out := make([]usbCameraDevice, 0)
	for _, line := range logs {
		for _, m := range re.FindAllStringSubmatch(line, -1) {
			idx, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			name := strings.TrimSpace(m[2])
			if name == "" {
				continue
			}
			// AVFoundation also exposes screen-capture "devices"; skip them.
			if strings.Contains(strings.ToLower(name), "capture screen") {
				continue
			}
			out = append(out, usbCameraDevice{
				Index: idx,
				Name:  name,
				UID:   name,
			})
		}
	}
	return out
}
