package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/asticode/go-astiav"
)

// ── decodeFrameToJPEG tests ─────────────────────────────────────────────────

func TestDecodeFrameToJPEG_SyntheticRGBA(t *testing.T) {
	w, h := 16, 16

	frame := astiav.AllocFrame()
	defer frame.Free()
	frame.SetWidth(w)
	frame.SetHeight(h)
	frame.SetPixelFormat(astiav.PixelFormatRgba)
	if err := frame.AllocBuffer(1); err != nil {
		t.Fatalf("alloc frame buffer: %v", err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: 0x33, G: 0x99, B: 0xcc, A: 0xff})
		}
	}
	if err := frame.Data().FromImage(img); err != nil {
		t.Fatalf("from image: %v", err)
	}

	jpegData, swsCtx, err := decodeFrameToJPEG(frame, nil)
	if swsCtx != nil {
		defer swsCtx.Free()
	}
	if err != nil {
		t.Fatalf("decodeFrameToJPEG: %v", err)
	}
	if len(jpegData) == 0 {
		t.Fatal("expected JPEG data, got empty bytes")
	}
	if jpegData[0] != 0xFF || jpegData[1] != 0xD8 {
		t.Fatalf("JPEG does not start with SOI marker: % X", jpegData[:min(len(jpegData), 4)])
	}
}

func TestDecodeFrameToJPEG_ZeroSizeFrame(t *testing.T) {
	frame := astiav.AllocFrame()
	defer frame.Free()
	frame.SetWidth(0)
	frame.SetHeight(0)
	frame.SetPixelFormat(astiav.PixelFormatRgba)

	jpegData, swsCtx, err := decodeFrameToJPEG(frame, nil)
	if swsCtx != nil {
		swsCtx.Free()
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jpegData != nil {
		t.Fatalf("expected nil for zero-size frame, got %d bytes", len(jpegData))
	}
}

func TestDecodeFrameToJPEG_SwsReuse(t *testing.T) {
	w, h := 8, 8
	frame := astiav.AllocFrame()
	defer frame.Free()
	frame.SetWidth(w)
	frame.SetHeight(h)
	frame.SetPixelFormat(astiav.PixelFormatRgba)
	if err := frame.AllocBuffer(1); err != nil {
		t.Fatalf("alloc buffer: %v", err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff})
		}
	}
	if err := frame.Data().FromImage(img); err != nil {
		t.Fatalf("from image: %v", err)
	}

	_, swsCtx1, err := decodeFrameToJPEG(frame, nil)
	if err != nil {
		t.Fatalf("first decode: %v", err)
	}
	defer swsCtx1.Free()

	_, swsCtx2, err := decodeFrameToJPEG(frame, swsCtx1)
	if err != nil {
		t.Fatalf("second decode: %v", err)
	}
	if swsCtx2 != nil {
		t.Error("expected swsCtx to be reused (nil returned)")
	}
}

// ── USBCameraRelay core tests ────────────────────────────────────────────────

func TestNewUSBCameraRelay(t *testing.T) {
	r := newUSBCameraRelay("usb:cam", "FaceTime HD Camera")
	if r.sourceID != "usb:cam" {
		t.Errorf("sourceID: got %q, want %q", r.sourceID, "usb:cam")
	}
	if r.cameraName != "FaceTime HD Camera" {
		t.Errorf("cameraName: got %q, want %q", r.cameraName, "FaceTime HD Camera")
	}
	if r.subscribers == nil {
		t.Error("subscribers map must not be nil")
	}
	if r.openDevice == nil {
		t.Error("openDevice must be set")
	}
	r.cancel()
}

func TestUSBCameraRelay_SubscribeBroadcast(t *testing.T) {
	r := newUSBCameraRelay("s1", "Cam")
	defer r.cancel()

	ch, unsub := r.subscribe()
	frame := []byte("jpeg-data")
	r.broadcast(frame)

	select {
	case got := <-ch:
		if string(got) != string(frame) {
			t.Errorf("broadcast: got %q, want %q", got, frame)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast frame")
	}

	unsub()
	r.broadcast([]byte("second"))
	select {
	case <-ch:
		t.Error("should not receive after unsubscribe")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestUSBCameraRelay_BroadcastDropsSlow(t *testing.T) {
	r := newUSBCameraRelay("s1", "Cam")
	defer r.cancel()

	ch, unsub := r.subscribe()
	defer unsub()

	r.broadcast([]byte("a"))
	r.broadcast([]byte("b"))
	r.broadcast([]byte("c")) // should be dropped

	if len(ch) != 2 {
		t.Errorf("expected 2 frames in channel, got %d", len(ch))
	}
}

func TestUSBCameraRelay_Stop(t *testing.T) {
	origLister := defaultDeviceLister
	defaultDeviceLister = &fakeDeviceLister{devs: []usbCameraDevice{{Index: 0, Name: "Cam"}}}
	defer func() { defaultDeviceLister = origLister }()

	r := newUSBCameraRelay("s1", "Cam")
	path := writeTestMJPEGFile(t, 1)
	r.openDevice = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		return openMJPEGFile(t, path)
	}
	_, err := r.Start(func(string, any) {})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.Stop()

	select {
	case <-r.ctx.Done():
	case <-time.After(500 * time.Millisecond):
		t.Error("context not cancelled after Stop")
	}
}

func TestUSBCameraRelay_Start_ReturnsValidPort(t *testing.T) {
	origLister := defaultDeviceLister
	defaultDeviceLister = &fakeDeviceLister{devs: []usbCameraDevice{{Index: 0, Name: "Cam"}}}
	defer func() { defaultDeviceLister = origLister }()

	r := newUSBCameraRelay("s1", "Cam")
	path := writeTestMJPEGFile(t, 1)
	r.openDevice = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		return openMJPEGFile(t, path)
	}
	port, err := r.Start(func(string, any) {})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Stop()

	if port <= 0 || port > 65535 {
		t.Fatalf("invalid port: %d", port)
	}

	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/healthz"
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", resp.StatusCode)
	}
}

func TestUSBCameraRelay_ServeMJPEG(t *testing.T) {
	r := newUSBCameraRelay("s1", "Cam")
	defer r.cancel()

	srv := httptest.NewServer(http.HandlerFunc(r.serveMJPEG))
	defer srv.Close()

	frame1 := []byte("fake-jpeg-1")
	frame2 := []byte("fake-jpeg-2")

	go func() {
		time.Sleep(20 * time.Millisecond)
		r.broadcast(frame1)
		time.Sleep(20 * time.Millisecond)
		r.broadcast(frame2)
		time.Sleep(20 * time.Millisecond)
		r.cancel()
	}()

	resp, err := http.Get(srv.URL) //nolint:noctx
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	mt, params, _ := mime.ParseMediaType(ct)
	if mt != "multipart/x-mixed-replace" {
		t.Fatalf("content-type: got %q, want multipart/x-mixed-replace", mt)
	}

	mr := multipart.NewReader(resp.Body, params["boundary"])
	var parts [][]byte
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		data, _ := io.ReadAll(p)
		parts = append(parts, data)
	}

	if len(parts) < 2 {
		t.Fatalf("expected ≥2 MJPEG parts, got %d", len(parts))
	}
	if string(parts[0]) != string(frame1) {
		t.Errorf("part 0: got %q, want %q", parts[0], frame1)
	}
	if string(parts[1]) != string(frame2) {
		t.Errorf("part 1: got %q, want %q", parts[1], frame2)
	}
}

func TestUSBCameraRelay_CaptureLoop_MJPEGFile(t *testing.T) {
	file := writeTestMJPEGFile(t, 2)

	r := newUSBCameraRelay("s1", "Cam")
	r.openDevice = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		return openMJPEGFile(t, file)
	}

	ch, unsub := r.subscribe()
	defer unsub()

	var emitted []string
	done := make(chan struct{})

	var frames [][]byte
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeout := time.NewTimer(500 * time.Millisecond)
		defer timeout.Stop()
		for {
			select {
			case f := <-ch:
				frames = append(frames, f)
			case <-done:
				return
			case <-timeout.C:
				return
			}
		}
	}()

	go func() {
		defer close(done)
		r.capture(0, func(name string, _ any) { emitted = append(emitted, name) })
	}()

	<-done
	r.cancel()
	wg.Wait()

	if len(frames) < 1 {
		t.Fatalf("expected at least 1 MJPEG frame, got %d", len(frames))
	}
	for _, f := range frames {
		if len(f) < 2 || f[0] != 0xFF || f[1] != 0xD8 {
			t.Errorf("frame does not start with JPEG SOI: % X", f[:min(len(f), 4)])
		}
	}

	for _, e := range emitted {
		if e == "usb:error" {
			t.Fatalf("unexpected usb:error event: %v", emitted)
		}
	}
}

func TestUSBCameraRelay_CaptureLoop_OpenError_EmitsError(t *testing.T) {
	r := newUSBCameraRelay("s1", "Cam")
	r.openDevice = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		return nil, nil, fmt.Errorf("permission denied")
	}

	var emitted []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.capture(0, func(name string, _ any) { emitted = append(emitted, name) })
	}()
	<-done

	found := false
	for _, e := range emitted {
		if e == "usb:error" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected usb:error event, got %v", emitted)
	}
}

func TestUSBCameraRelay_emitError_DeviceNotFound(t *testing.T) {
	r := newUSBCameraRelay("s1", "Cam")
	var emitted []string
	r.emitError(fmt.Errorf("no such file or directory"), func(name string, _ any) { emitted = append(emitted, name) })

	found := false
	for _, e := range emitted {
		if e == "usb:error" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected usb:error event, got %v", emitted)
	}
}

func TestUSBCameraRelay_emitError_ContextCancelled(t *testing.T) {
	r := newUSBCameraRelay("s1", "Cam")
	r.cancel()

	var emitted []string
	r.emitError(fmt.Errorf("permission denied"), func(name string, _ any) { emitted = append(emitted, name) })

	if len(emitted) != 0 {
		t.Fatalf("expected no event after context cancel, got %v", emitted)
	}
}

func TestUSBCameraRelay_CaptureLoop_NotVideo(t *testing.T) {
	file := writeTestMJPEGFile(t, 1)
	r := newUSBCameraRelay("s1", "Cam")
	r.openDevice = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		formatCtx, stream, err := openMJPEGFile(t, file)
		if err != nil {
			return nil, nil, err
		}
		stream.CodecParameters().SetMediaType(astiav.MediaTypeAudio)
		return formatCtx, stream, nil
	}

	var emitted []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.capture(0, func(name string, _ any) { emitted = append(emitted, name) })
	}()
	<-done

	found := false
	for _, e := range emitted {
		if e == "usb:error" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected usb:error event for non-video stream, got %v", emitted)
	}
}

func TestUSBCameraRelay_CaptureLoop_NoDecoder(t *testing.T) {
	file := writeTestMJPEGFile(t, 1)
	r := newUSBCameraRelay("s1", "Cam")
	r.openDevice = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		formatCtx, stream, err := openMJPEGFile(t, file)
		if err != nil {
			return nil, nil, err
		}
		stream.CodecParameters().SetCodecID(astiav.CodecIDNone)
		return formatCtx, stream, nil
	}

	var emitted []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.capture(0, func(name string, _ any) { emitted = append(emitted, name) })
	}()
	<-done

	found := false
	for _, e := range emitted {
		if e == "usb:error" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected usb:error event when decoder is missing, got %v", emitted)
	}
}

func TestDecodeFrameToJPEG_NoPixelFormat(t *testing.T) {
	frame := astiav.AllocFrame()
	defer frame.Free()
	frame.SetWidth(16)
	frame.SetHeight(16)
	frame.SetPixelFormat(astiav.PixelFormatNone)

	_, swsCtx, err := decodeFrameToJPEG(frame, nil)
	if swsCtx != nil {
		swsCtx.Free()
	}
	if err == nil {
		t.Fatal("expected error for frame with no pixel format")
	}
}

func TestDecodeFrameToJPEG_DimensionChange(t *testing.T) {
	w, h := 8, 8
	frame := astiav.AllocFrame()
	defer frame.Free()
	frame.SetWidth(w)
	frame.SetHeight(h)
	frame.SetPixelFormat(astiav.PixelFormatRgba)
	if err := frame.AllocBuffer(1); err != nil {
		t.Fatalf("alloc buffer: %v", err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if err := frame.Data().FromImage(img); err != nil {
		t.Fatalf("from image: %v", err)
	}

	_, swsCtx1, err := decodeFrameToJPEG(frame, nil)
	if err != nil {
		t.Fatalf("first decode: %v", err)
	}

	frame2 := astiav.AllocFrame()
	defer frame2.Free()
	frame2.SetWidth(16)
	frame2.SetHeight(16)
	frame2.SetPixelFormat(astiav.PixelFormatRgba)
	if err := frame2.AllocBuffer(1); err != nil {
		t.Fatalf("alloc buffer: %v", err)
	}
	img2 := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	if err := frame2.Data().FromImage(img2); err != nil {
		t.Fatalf("from image: %v", err)
	}

	_, swsCtx2, err := decodeFrameToJPEG(frame2, swsCtx1)
	if err != nil {
		t.Fatalf("second decode: %v", err)
	}
	if swsCtx2 == nil {
		t.Fatal("expected swsCtx to be recreated for different dimensions")
	}
	swsCtx2.Free()
}

func TestDecodeFrameToJPEG_PixelFormatChange(t *testing.T) {
	w, h := 16, 16
	frame := astiav.AllocFrame()
	defer frame.Free()
	frame.SetWidth(w)
	frame.SetHeight(h)
	frame.SetPixelFormat(astiav.PixelFormatRgba)
	if err := frame.AllocBuffer(1); err != nil {
		t.Fatalf("alloc buffer: %v", err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if err := frame.Data().FromImage(img); err != nil {
		t.Fatalf("from image: %v", err)
	}

	_, swsCtx1, err := decodeFrameToJPEG(frame, nil)
	if err != nil {
		t.Fatalf("first decode: %v", err)
	}

	frame2 := astiav.AllocFrame()
	defer frame2.Free()
	frame2.SetWidth(w)
	frame2.SetHeight(h)
	frame2.SetPixelFormat(astiav.PixelFormatYuv420P)
	if err := frame2.AllocBuffer(1); err != nil {
		t.Fatalf("alloc buffer: %v", err)
	}

	_, swsCtx2, err := decodeFrameToJPEG(frame2, swsCtx1)
	if err != nil {
		t.Fatalf("yuv decode: %v", err)
	}
	if swsCtx2 == nil {
		t.Fatal("expected swsCtx to be recreated for different pixel format")
	}
	swsCtx2.Free()
}

func TestUSBCameraRelay_Start_ResolveErrors(t *testing.T) {
	r := newUSBCameraRelay("s1", "Cam")

	origLister := defaultDeviceLister
	defaultDeviceLister = &fakeDeviceLister{err: fmt.Errorf("no cameras")}
	defer func() { defaultDeviceLister = origLister }()

	_, err := r.Start(func(string, any) {})
	if err == nil {
		t.Fatal("expected error when listing devices fails")
	}

	defaultDeviceLister = &fakeDeviceLister{devs: []usbCameraDevice{{Index: 0, Name: "Other"}}}
	_, err = r.Start(func(string, any) {})
	if err == nil {
		t.Fatal("expected error when camera name cannot be resolved")
	}
}

func TestDecodePacket_H264(t *testing.T) {
	sps, pps, idr := encodeTestH264Frame(t)
	if len(sps) == 0 || len(pps) == 0 || len(idr) == 0 {
		t.Skip("H.264 test encoder unavailable")
	}

	file := writeH264TestFile(t, sps, pps, idr)

	formatCtx := astiav.AllocFormatContext()
	if formatCtx == nil {
		t.Fatal("alloc format context")
	}
	defer formatCtx.Free()

	inputFormat := astiav.FindInputFormat("h264")
	if inputFormat == nil {
		t.Skip("h264 input format unavailable")
	}
	if err := formatCtx.OpenInput(file, inputFormat, nil); err != nil {
		t.Fatalf("open h264 file: %v", err)
	}
	defer formatCtx.CloseInput()

	if err := formatCtx.FindStreamInfo(nil); err != nil {
		t.Fatalf("find stream info: %v", err)
	}

	stream, _, err := formatCtx.FindBestStream(astiav.MediaTypeVideo, -1, -1)
	if err != nil {
		t.Fatalf("find best stream: %v", err)
	}

	decoder := astiav.FindDecoder(stream.CodecParameters().CodecID())
	if decoder == nil {
		t.Skip("h264 decoder unavailable")
	}

	r := newUSBCameraRelay("s1", "Cam")
	defer r.Stop()
	r.decoderCtx = astiav.AllocCodecContext(decoder)
	if r.decoderCtx == nil {
		t.Fatal("alloc decoder context")
	}
	if err := stream.CodecParameters().ToCodecContext(r.decoderCtx); err != nil {
		t.Fatalf("copy codec parameters: %v", err)
	}
	if err := r.decoderCtx.Open(decoder, nil); err != nil {
		t.Fatalf("open decoder: %v", err)
	}

	var au bytes.Buffer
	au.Write([]byte{0x00, 0x00, 0x00, 0x01})
	au.Write(idr)

	packet := astiav.AllocPacket()
	defer packet.Free()
	if err := packet.FromData(au.Bytes()); err != nil {
		t.Fatalf("packet from data: %v", err)
	}
	packet.SetStreamIndex(stream.Index())

	frame := astiav.AllocFrame()
	defer frame.Free()

	jpeg, err := r.decodePacket(packet, frame)
	if err != nil {
		t.Fatalf("decodePacket: %v", err)
	}
	if len(jpeg) < 2 || jpeg[0] != 0xFF || jpeg[1] != 0xD8 {
		t.Fatalf("expected JPEG SOI, got % X", jpeg[:min(len(jpeg), 4)])
	}

	r.Stop()
	if r.swsCtx != nil {
		t.Fatal("software scaling context should be nil after Stop")
	}
	if r.decoderCtx != nil {
		r.decoderCtx.Free()
		r.decoderCtx = nil
	}
}

func TestUSBCameraRelay_CaptureLoop_H264File(t *testing.T) {
	sps, pps, idr := encodeTestH264Frame(t)
	if len(sps) == 0 || len(pps) == 0 || len(idr) == 0 {
		t.Skip("H.264 test encoder unavailable")
	}

	file := writeH264TestFile(t, sps, pps, idr)

	r := newUSBCameraRelay("s1", "Cam")
	r.openDevice = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		return openH264File(t, file)
	}

	ch, unsub := r.subscribe()
	defer unsub()

	var frames [][]byte
	var emitted []string
	done := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeout := time.NewTimer(2 * time.Second)
		defer timeout.Stop()
		for {
			select {
			case f := <-ch:
				frames = append(frames, f)
			case <-done:
				return
			case <-timeout.C:
				return
			}
		}
	}()

	go func() {
		defer close(done)
		r.capture(0, func(name string, _ any) { emitted = append(emitted, name) })
	}()

	<-done
	r.cancel()
	wg.Wait()

	if len(frames) < 1 {
		t.Fatalf("expected at least 1 decoded frame, got %d", len(frames))
	}
	for _, f := range frames {
		if len(f) < 2 || f[0] != 0xFF || f[1] != 0xD8 {
			t.Errorf("frame does not start with JPEG SOI: % X", f[:min(len(f), 4)])
		}
	}

	for _, e := range emitted {
		if e == "usb:error" {
			t.Fatalf("unexpected usb:error event: %v", emitted)
		}
	}
}

func TestDecodePacket_Invalid(t *testing.T) {
	r := newUSBCameraRelay("s1", "Cam")
	packet := astiav.AllocPacket()
	defer packet.Free()
	if err := packet.FromData([]byte{0x00, 0x00, 0x00, 0x01, 0xFF}); err != nil {
		t.Fatalf("packet from data: %v", err)
	}
	frame := astiav.AllocFrame()
	defer frame.Free()

	_, err := r.decodePacket(packet, frame)
	if err == nil {
		t.Fatal("expected error for invalid packet without decoder")
	}
}

func TestDecodePacket_H264Invalid(t *testing.T) {
	sps, pps, idr := encodeTestH264Frame(t)
	if len(sps) == 0 || len(pps) == 0 || len(idr) == 0 {
		t.Skip("H.264 test encoder unavailable")
	}

	file := writeH264TestFile(t, sps, pps, idr)
	formatCtx := astiav.AllocFormatContext()
	if formatCtx == nil {
		t.Fatal("alloc format context")
	}
	defer formatCtx.Free()

	inputFormat := astiav.FindInputFormat("h264")
	if inputFormat == nil {
		t.Skip("h264 input format unavailable")
	}
	if err := formatCtx.OpenInput(file, inputFormat, nil); err != nil {
		t.Fatalf("open h264 file: %v", err)
	}
	defer formatCtx.CloseInput()
	if err := formatCtx.FindStreamInfo(nil); err != nil {
		t.Fatalf("find stream info: %v", err)
	}

	stream, _, err := formatCtx.FindBestStream(astiav.MediaTypeVideo, -1, -1)
	if err != nil {
		t.Fatalf("find best stream: %v", err)
	}

	decoder := astiav.FindDecoder(stream.CodecParameters().CodecID())
	if decoder == nil {
		t.Skip("h264 decoder unavailable")
	}

	r := newUSBCameraRelay("s1", "Cam")
	defer r.Stop()
	r.decoderCtx = astiav.AllocCodecContext(decoder)
	if r.decoderCtx == nil {
		t.Fatal("alloc decoder context")
	}
	if err := stream.CodecParameters().ToCodecContext(r.decoderCtx); err != nil {
		t.Fatalf("copy codec parameters: %v", err)
	}
	if err := r.decoderCtx.Open(decoder, nil); err != nil {
		t.Fatalf("open decoder: %v", err)
	}

	packet := astiav.AllocPacket()
	defer packet.Free()
	if err := packet.FromData([]byte{0x00, 0x00, 0x00, 0x01, 0xFF}); err != nil {
		t.Fatalf("packet from data: %v", err)
	}
	packet.SetStreamIndex(stream.Index())

	frame := astiav.AllocFrame()
	defer frame.Free()

	_, err = r.decodePacket(packet, frame)
	if err == nil {
		t.Fatal("expected error for invalid H.264 NALU")
	}
}

// ── AppService USB tests ───────────────────────────────────────────────────

func TestAppService_StartUSBCamera_MissingSource(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "cfg.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "rtsp:s1", Type: "rtsp", Name: "Stream", URL: "rtsp://x"}}})
	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay)}

	_, err := a.StartUSBCamera("rtsp:s1")
	if err == nil {
		t.Fatal("expected error for non-USB source")
	}
}

func TestAppService_StartUSBCamera_WrongType(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "cfg.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "rtsp:s1", Type: "rtsp", Name: "Stream", URL: "rtsp://x"}}})
	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay)}

	_, err := a.StartUSBCamera("rtsp:s1")
	if err == nil {
		t.Fatal("expected error for non-USB source")
	}
}

func TestAppService_StartUSBCamera_RenamedSourceUsesSystemName(t *testing.T) {
	origLister := defaultDeviceLister
	defaultDeviceLister = &fakeDeviceLister{devs: []usbCameraDevice{{Index: 0, Name: "System Camera"}}}
	origOpen := openUSBCamera
	path := writeTestMJPEGFile(t, 1)
	openUSBCamera = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		return openMJPEGFile(t, path)
	}
	defer func() {
		defaultDeviceLister = origLister
		openUSBCamera = origOpen
	}()

	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "cfg.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "usb:cam", Type: "usb", Name: "System Camera", DisplayName: "Lectern Camera"}}})
	a := NewAppService()
	a.config = cm

	if _, err := a.StartUSBCamera("usb:cam"); err != nil {
		t.Fatalf("StartUSBCamera: %v", err)
	}
	if got := a.usbRelays["usb:cam"].cameraName; got != "System Camera" {
		t.Fatalf("relay camera name = %q, want system device name", got)
	}
	if err := a.StopUSBCamera("usb:cam"); err != nil {
		t.Fatalf("StopUSBCamera: %v", err)
	}
}

func TestAppService_StartUSBCamera_FakeDevice(t *testing.T) {
	origLister := defaultDeviceLister
	defaultDeviceLister = &fakeDeviceLister{devs: []usbCameraDevice{{Index: 0, Name: "Cam"}}}
	origOpen := openUSBCamera
	path := writeTestMJPEGFile(t, 1)
	openUSBCamera = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		return openMJPEGFile(t, path)
	}
	defer func() {
		defaultDeviceLister = origLister
		openUSBCamera = origOpen
	}()

	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "cfg.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "usb:cam", Type: "usb", Name: "Cam"}}})
	a := NewAppService()
	a.config = cm

	port, err := a.StartUSBCamera("usb:cam")
	if err != nil {
		t.Fatalf("StartUSBCamera: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("invalid port: %d", port)
	}

	if err := a.StopUSBCamera("usb:cam"); err != nil {
		t.Fatalf("StopUSBCamera: %v", err)
	}
	if len(a.usbRelays) != 0 {
		t.Fatalf("expected USB relay removed after stop, got %d", len(a.usbRelays))
	}
}

func TestAppService_StopUSBCamera(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "cfg.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "usb:cam", Type: "usb", Name: "Cam"}}})
	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay), usbRelays: make(map[string]*USBCameraRelay)}

	r := newUSBCameraRelay("usb:cam", "Cam")
	r.port = 12345
	a.usbRelays["usb:cam"] = r

	if err := a.StopUSBCamera("usb:cam"); err != nil {
		t.Fatalf("StopUSBCamera: %v", err)
	}
	if len(a.usbRelays) != 0 {
		t.Errorf("expected relay removed, got %d", len(a.usbRelays))
	}
}

func TestAppService_ServiceShutdown_StopsUSBRelays(t *testing.T) {
	origLister := defaultDeviceLister
	defaultDeviceLister = &fakeDeviceLister{devs: []usbCameraDevice{{Index: 0, Name: "Cam"}}}
	defer func() { defaultDeviceLister = origLister }()

	dir := t.TempDir()
	a := &AppService{
		config:    &ConfigManager{path: filepath.Join(dir, "cfg.json")},
		relays:    make(map[string]*RTSPRelay),
		usbRelays: make(map[string]*USBCameraRelay),
	}
	r := newUSBCameraRelay("usb:cam", "Cam")
	path := writeTestMJPEGFile(t, 1)
	r.openDevice = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		return openMJPEGFile(t, path)
	}
	_, err := r.Start(func(string, any) {})
	if err != nil {
		t.Fatalf("relay Start: %v", err)
	}
	a.usbRelays["usb:cam"] = r

	if err := a.ServiceShutdown(); err != nil {
		t.Fatalf("ServiceShutdown: %v", err)
	}
	if len(a.usbRelays) != 0 {
		t.Errorf("expected 0 USB relays after shutdown, got %d", len(a.usbRelays))
	}
	select {
	case <-r.ctx.Done():
	case <-time.After(500 * time.Millisecond):
		t.Error("relay context not cancelled after ServiceShutdown")
	}
}

func TestUSBCameraRelay_StopWaitsForCapture(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	r := newUSBCameraRelay("usb:cam", "Cam")
	r.running.Add(1)
	go func() {
		defer r.running.Done()
		close(entered)
		<-release
	}()
	<-entered

	stopped := make(chan struct{})
	go func() {
		r.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
		t.Fatal("Stop returned while capture was still blocked")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after capture exited")
	}
}

func TestAppService_StartUSBCamera_ExistingRelay(t *testing.T) {
	origLister := defaultDeviceLister
	defaultDeviceLister = &fakeDeviceLister{devs: []usbCameraDevice{{Index: 0, Name: "Cam"}}}
	origOpen := openUSBCamera
	path := writeTestMJPEGFile(t, 1)
	openUSBCamera = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		return openMJPEGFile(t, path)
	}
	defer func() {
		defaultDeviceLister = origLister
		openUSBCamera = origOpen
	}()

	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "cfg.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "usb:cam", Type: "usb", Name: "Cam"}}})
	a := NewAppService()
	a.config = cm

	port1, err := a.StartUSBCamera("usb:cam")
	if err != nil {
		t.Fatalf("StartUSBCamera first: %v", err)
	}
	firstRelay := a.usbRelays["usb:cam"]
	port2, err := a.StartUSBCamera("usb:cam")
	if err != nil {
		t.Fatalf("StartUSBCamera second: %v", err)
	}
	if port1 == port2 {
		t.Fatalf("expected different ports after restart, got %d", port1)
	}
	if len(a.usbRelays) != 1 {
		t.Fatalf("expected 1 USB relay, got %d", len(a.usbRelays))
	}
	if a.usbRelays["usb:cam"] == firstRelay {
		t.Fatal("expected the restarted relay to be active")
	}
	if err := a.StopUSBCamera("usb:cam"); err != nil {
		t.Fatalf("StopUSBCamera: %v", err)
	}
}

func TestAppService_StartUSBCamera_ResolveError(t *testing.T) {
	origLister := defaultDeviceLister
	defaultDeviceLister = &fakeDeviceLister{devs: []usbCameraDevice{{Index: 0, Name: "Unrelated Lens"}}}
	defer func() { defaultDeviceLister = origLister }()

	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "cfg.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "usb:cam", Type: "usb", Name: "Cam"}}})
	a := NewAppService()
	a.config = cm

	_, err := a.StartUSBCamera("usb:cam")
	if err == nil {
		t.Fatal("expected error when device resolution fails")
	}
	if len(a.usbRelays) != 0 {
		t.Fatalf("expected no relay after resolution failure, got %d", len(a.usbRelays))
	}
}

func TestAppService_StartUSBCamera_OpenError(t *testing.T) {
	origLister := defaultDeviceLister
	defaultDeviceLister = &fakeDeviceLister{devs: []usbCameraDevice{{Index: 0, Name: "Cam"}}}
	origOpen := openUSBCamera
	openUSBCamera = func(int) (*astiav.FormatContext, *astiav.Stream, error) {
		return nil, nil, fmt.Errorf("permission denied")
	}
	defer func() {
		defaultDeviceLister = origLister
		openUSBCamera = origOpen
	}()

	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "cfg.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "usb:cam", Type: "usb", Name: "Cam"}}})
	a := NewAppService()
	a.config = cm

	if _, err := a.StartUSBCamera("usb:cam"); err == nil {
		t.Fatal("expected camera open error")
	}
	if len(a.usbRelays) != 0 {
		t.Fatalf("expected no relay after open failure, got %d", len(a.usbRelays))
	}
}

func TestAppService_StopUSBCamera_NilMap(t *testing.T) {
	a := &AppService{}
	if err := a.StopUSBCamera("usb:cam"); err != nil {
		t.Fatalf("StopUSBCamera: %v", err)
	}
}

func TestInitLogger(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	initLogger()
}

func TestAppService_GetLogPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	a := NewAppService()
	path, err := a.GetLogPath()
	if err != nil {
		t.Fatalf("GetLogPath: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty log path")
	}
}

func TestAppService_RevealLogsInFinder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	a := NewAppService()
	// This may open a Finder window on macOS; the test simply verifies no panic.
	_ = a.RevealLogsInFinder()
}

func TestAppService_GetUSBCameras(t *testing.T) {
	a := NewAppService()
	_, _ = a.GetUSBCameras()
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func writeTestMJPEGFile(t *testing.T, count int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			img.Set(x, y, color.RGBA{R: 0x33, G: 0x99, B: 0xcc, A: 0xff})
		}
	}
	var buf bytes.Buffer
	for range count {
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70}); err != nil {
			t.Fatalf("encode jpeg: %v", err)
		}
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.mjpeg")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write mjpeg file: %v", err)
	}
	return path
}

func writeH264TestFile(t *testing.T, sps, pps, idr []byte) string {
	t.Helper()
	startCode := []byte{0x00, 0x00, 0x00, 0x01}

	var buf bytes.Buffer
	buf.Write(startCode)
	buf.Write(sps)
	buf.Write(startCode)
	buf.Write(pps)
	buf.Write(startCode)
	buf.Write(idr)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.264")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write h264 file: %v", err)
	}
	return path
}

func openMJPEGFile(t *testing.T, path string) (*astiav.FormatContext, *astiav.Stream, error) {
	t.Helper()
	formatCtx := astiav.AllocFormatContext()
	if formatCtx == nil {
		return nil, nil, fmt.Errorf("alloc format context")
	}

	inputFormat := astiav.FindInputFormat("mjpeg")
	if inputFormat == nil {
		formatCtx.Free()
		return nil, nil, fmt.Errorf("mjpeg input format not available")
	}

	if err := formatCtx.OpenInput(path, inputFormat, nil); err != nil {
		formatCtx.Free()
		return nil, nil, err
	}

	// For raw MJPEG, OpenInput already creates a stream; avoid FindStreamInfo so
	// the read pointer stays at the beginning of the file.
	if formatCtx.NbStreams() > 0 {
		return formatCtx, formatCtx.Streams()[0], nil
	}

	if err := formatCtx.FindStreamInfo(nil); err != nil {
		formatCtx.CloseInput()
		formatCtx.Free()
		return nil, nil, err
	}

	stream, _, err := formatCtx.FindBestStream(astiav.MediaTypeVideo, -1, -1)
	if err != nil {
		formatCtx.CloseInput()
		formatCtx.Free()
		return nil, nil, err
	}
	return formatCtx, stream, nil
}

func openH264File(t *testing.T, path string) (*astiav.FormatContext, *astiav.Stream, error) {
	t.Helper()
	formatCtx := astiav.AllocFormatContext()
	if formatCtx == nil {
		return nil, nil, fmt.Errorf("alloc format context")
	}

	inputFormat := astiav.FindInputFormat("h264")
	if inputFormat == nil {
		formatCtx.Free()
		return nil, nil, fmt.Errorf("h264 input format not available")
	}

	if err := formatCtx.OpenInput(path, inputFormat, nil); err != nil {
		formatCtx.Free()
		return nil, nil, err
	}

	if err := formatCtx.FindStreamInfo(nil); err != nil {
		formatCtx.CloseInput()
		formatCtx.Free()
		return nil, nil, err
	}

	// Rewind to the beginning so ReadFrame can produce all frames in the file.
	if err := formatCtx.SeekFrame(-1, 0, astiav.NewSeekFlags(astiav.SeekFlagByte)); err != nil {
		formatCtx.CloseInput()
		formatCtx.Free()
		return nil, nil, err
	}

	stream, _, err := formatCtx.FindBestStream(astiav.MediaTypeVideo, -1, -1)
	if err != nil {
		formatCtx.CloseInput()
		formatCtx.Free()
		return nil, nil, err
	}
	return formatCtx, stream, nil
}
