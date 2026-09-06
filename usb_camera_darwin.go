//go:build darwin
// +build darwin

package main

/*
#cgo CFLAGS: -I/opt/homebrew/opt/ffmpeg/include
#include <libavdevice/avdevice.h>
#include <libavformat/avformat.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    int index;
    char *name;
    char *description;
} device_info;

int list_avfoundation_devices(device_info **out, int *count) {
    avdevice_register_all();

    AVInputFormat *fmt = (AVInputFormat *)av_find_input_format("avfoundation");
    if (!fmt) {
        return AVERROR(ENOSYS);
    }

    AVFormatContext *ctx = avformat_alloc_context();
    if (!ctx) {
        return AVERROR(ENOMEM);
    }

    ctx->iformat = (AVInputFormat *)fmt;

    AVDeviceInfoList *list = NULL;
    int ret = avdevice_list_devices(ctx, &list);
    if (ret < 0) {
        avformat_free_context(ctx);
        return ret;
    }

    int n = list->nb_devices;
    if (n > 0) {
        *out = (device_info *)malloc(sizeof(device_info) * n);
        if (!*out) {
            avdevice_free_list_devices(&list);
            avformat_free_context(ctx);
            return AVERROR(ENOMEM);
        }
        for (int i = 0; i < n; i++) {
            AVDeviceInfo *dev = list->devices[i];
            (*out)[i].index = i;
            (*out)[i].name = dev->device_name ? strdup(dev->device_name) : NULL;
            (*out)[i].description = dev->device_description ? strdup(dev->device_description) : NULL;
        }
    } else {
        *out = NULL;
    }

    *count = n;
    avdevice_free_list_devices(&list);
    avformat_free_context(ctx);
    return 0;
}

void free_device_infos(device_info *devs, int count) {
    if (!devs) return;
    for (int i = 0; i < count; i++) {
        free(devs[i].name);
        free(devs[i].description);
    }
    free(devs);
}
*/
import "C"

import (
	"fmt"
	"unsafe"

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

func (l *avfDeviceLister) listDevices() ([]usbCameraDevice, error) {
	var cDevs *C.device_info
	var cCount C.int

	ret := C.list_avfoundation_devices(&cDevs, &cCount)
	if ret < 0 {
		return nil, fmt.Errorf("list AVFoundation devices: %w", astiav.Error(ret))
	}
	defer C.free_device_infos(cDevs, cCount)

	count := int(cCount)
	if count == 0 {
		return nil, nil
	}

	devs := (*[1 << 20]C.device_info)(unsafe.Pointer(cDevs))[:count:count]
	out := make([]usbCameraDevice, 0, count)
	for _, d := range devs {
		name := ""
		if d.description != nil {
			name = C.GoString(d.description)
		} else if d.name != nil {
			name = C.GoString(d.name)
		}
		uid := ""
		if d.name != nil {
			uid = C.GoString(d.name)
		}
		out = append(out, usbCameraDevice{
			Index: int(d.index),
			Name:  name,
			UID:   uid,
		})
	}
	return out, nil
}
