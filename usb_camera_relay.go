package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"github.com/asticode/go-astiav"
)

// USBCameraRelay captures frames from a macOS AVFoundation device, decodes them,
// and serves an MJPEG stream over a local HTTP server. It mirrors the interface of
// RTSPRelay so AppService can manage both source types uniformly.
type USBCameraRelay struct {
	sourceID   string
	cameraName string

	ctx    context.Context
	cancel context.CancelFunc

	port   int
	server *http.Server

	// subscriber broadcast
	mu          sync.Mutex
	subscribers map[chan []byte]struct{}

	// decoder state
	decoderCtx *astiav.CodecContext
	swsCtx     *astiav.SoftwareScaleContext
	running    sync.WaitGroup

	// openDevice is injectable for testing.
	openDevice func(deviceIndex int) (*astiav.FormatContext, *astiav.Stream, error)
}

// newUSBCameraRelay allocates a USB camera relay without starting it.
func newUSBCameraRelay(sourceID, cameraName string) *USBCameraRelay {
	ctx, cancel := context.WithCancel(context.Background())
	return &USBCameraRelay{
		sourceID:    sourceID,
		cameraName:  cameraName,
		ctx:         ctx,
		cancel:      cancel,
		subscribers: make(map[chan []byte]struct{}),
		openDevice:  openUSBCamera,
	}
}

// Start resolves the configured camera name to an AVFoundation device index,
// starts the MJPEG HTTP server, and launches the capture goroutine. It returns
// the localhost port on which the MJPEG stream is available.
func (r *USBCameraRelay) Start(emitEvent func(name string, data any)) (int, error) {
	devs, err := defaultDeviceLister.listDevices()
	if err != nil {
		log.Printf("usb camera list devices error (source=%s): %v", r.sourceID, err)
		return 0, fmt.Errorf("list USB cameras: %w", err)
	}
	idx, err := resolveUSBCamera(r.cameraName, devs)
	if err != nil {
		log.Printf("usb camera resolve error (source=%s, camera=%q): %v", r.sourceID, r.cameraName, err)
		return 0, fmt.Errorf("resolve camera %q: %w", r.cameraName, err)
	}
	formatCtx, stream, err := r.openDevice(idx)
	if err != nil {
		return 0, fmt.Errorf("open camera: %w", err)
	}
	if formatCtx == nil || stream == nil {
		if formatCtx != nil {
			formatCtx.CloseInput()
			formatCtx.Free()
		}
		return 0, fmt.Errorf("open camera: missing format context or video stream")
	}

	// Pick a free port and start the HTTP server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		formatCtx.CloseInput()
		formatCtx.Free()
		return 0, fmt.Errorf("listen: %w", err)
	}
	r.port = ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", r.serveMJPEG)
	r.server = &http.Server{Handler: mux}
	go func() {
		if err := r.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("usb camera relay http server error (source=%s): %v", r.sourceID, err)
		}
	}()

	// Launch capture in a goroutine so Start can return the port immediately.
	r.running.Add(1)
	go func() {
		defer r.running.Done()
		if err := r.captureOpened(formatCtx, stream); err != nil {
			log.Printf("usb camera capture error (source=%s): %v", r.sourceID, err)
			r.emitError(err, emitEvent)
		}
	}()

	return r.port, nil
}

// Stop cancels the relay context, closes the HTTP server, and frees decoder state.
func (r *USBCameraRelay) Stop() {
	r.cancel()
	if r.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = r.server.Shutdown(ctx)
	}
	r.running.Wait()
	if r.swsCtx != nil {
		r.swsCtx.Free()
		r.swsCtx = nil
	}
}

// subscribe registers a new MJPEG client and returns a frame channel and unsubscribe func.
func (r *USBCameraRelay) subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 2)
	r.mu.Lock()
	r.subscribers[ch] = struct{}{}
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		delete(r.subscribers, ch)
		r.mu.Unlock()
	}
}

// broadcast sends the latest JPEG frame to all subscribed clients, dropping slow readers.
func (r *USBCameraRelay) broadcast(frame []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for ch := range r.subscribers {
		select {
		case ch <- frame:
		default:
		}
	}
}

// serveMJPEG streams JPEG frames to an HTTP client via multipart/x-mixed-replace.
func (r *USBCameraRelay) serveMJPEG(w http.ResponseWriter, req *http.Request) {
	frameCh, unsubscribe := r.subscribe()
	defer unsubscribe()

	mw := multipart.NewWriter(w)
	boundary := "mjpegboundary"
	mw.SetBoundary(boundary) //nolint:errcheck

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		select {
		case frame := <-frameCh:
			h := make(textproto.MIMEHeader)
			h.Set("Content-Type", "image/jpeg")
			pw, err := mw.CreatePart(h)
			if err != nil {
				return
			}
			if _, err := pw.Write(frame); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-req.Context().Done():
			return
		case <-r.ctx.Done():
			return
		}
	}
}

// capture opens the AVFoundation device and decodes/broadcasts frames until the
// relay context is cancelled or a fatal error occurs.
func (r *USBCameraRelay) capture(deviceIndex int, emitEvent func(name string, data any)) {
	if err := r.captureLoop(deviceIndex, emitEvent); err != nil {
		log.Printf("usb camera capture error (source=%s): %v", r.sourceID, err)
		r.emitError(err, emitEvent)
	}
}

// captureLoop is the inner, testable capture implementation.
func (r *USBCameraRelay) captureLoop(deviceIndex int, emitEvent func(name string, data any)) error {
	formatCtx, stream, err := r.openDevice(deviceIndex)
	if err != nil {
		return fmt.Errorf("open camera: %w", err)
	}
	return r.captureOpened(formatCtx, stream)
}

func (r *USBCameraRelay) captureOpened(formatCtx *astiav.FormatContext, stream *astiav.Stream) error {
	defer func() {
		if formatCtx != nil {
			formatCtx.CloseInput()
			formatCtx.Free()
		}
	}()

	if stream.CodecParameters().MediaType() != astiav.MediaTypeVideo {
		return fmt.Errorf("selected stream is not video")
	}

	codecID := stream.CodecParameters().CodecID()
	isMJPEG := codecID == astiav.CodecIDMjpeg || codecID == astiav.CodecIDMjpegb

	if !isMJPEG {
		decoder := astiav.FindDecoder(codecID)
		if decoder == nil {
			return fmt.Errorf("no decoder for codec %q", codecID.String())
		}
		r.decoderCtx = astiav.AllocCodecContext(decoder)
		if r.decoderCtx == nil {
			return fmt.Errorf("allocate decoder context")
		}
		if err := stream.CodecParameters().ToCodecContext(r.decoderCtx); err != nil {
			return fmt.Errorf("copy codec parameters: %w", err)
		}
		if err := r.decoderCtx.Open(decoder, nil); err != nil {
			return fmt.Errorf("open decoder: %w", err)
		}
		defer func() {
			if r.decoderCtx != nil {
				r.decoderCtx.Free()
				r.decoderCtx = nil
			}
		}()
	}

	packet := astiav.AllocPacket()
	if packet == nil {
		return fmt.Errorf("allocate packet")
	}
	defer func() {
		if packet != nil {
			packet.Free()
		}
	}()
	frame := astiav.AllocFrame()
	if frame == nil {
		return fmt.Errorf("allocate frame")
	}
	defer func() {
		if frame != nil {
			frame.Free()
		}
	}()

	for {
		select {
		case <-r.ctx.Done():
			return nil
		default:
		}

		packet.Unref()
		if err := formatCtx.ReadFrame(packet); err != nil {
			if errors.Is(err, astiav.ErrEof) || errors.Is(r.ctx.Err(), context.Canceled) {
				return nil
			}
			if errors.Is(err, astiav.ErrEagain) {
				// Live capture devices frequently return EAGAIN between frames.
				time.Sleep(20 * time.Millisecond)
				continue
			}
			return fmt.Errorf("read frame: %w", err)
		}

		if packet.StreamIndex() != stream.Index() {
			continue
		}

		var jpegData []byte
		if isMJPEG {
			jpegData = bytes.Clone(packet.Data())
		} else {
			decoded, err := r.decodePacket(packet, frame)
			if err != nil {
				log.Printf("usb decode error (source=%s): %v", r.sourceID, err)
				continue
			}
			jpegData = decoded
		}

		if len(jpegData) > 0 {
			r.broadcast(jpegData)
		}
	}
}

// decodePacket sends a packet through the decoder and returns the resulting JPEG bytes.
func (r *USBCameraRelay) decodePacket(packet *astiav.Packet, frame *astiav.Frame) ([]byte, error) {
	if r.decoderCtx == nil {
		return nil, fmt.Errorf("decoder not initialized")
	}

	if err := r.decoderCtx.SendPacket(packet); err != nil {
		if errors.Is(err, astiav.ErrEagain) {
			return nil, nil
		}
		return nil, fmt.Errorf("send packet: %w", err)
	}

	for {
		frame.Unref()
		err := r.decoderCtx.ReceiveFrame(frame)
		if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("receive frame: %w", err)
		}

		jpeg, swsNew, err := decodeFrameToJPEG(frame, r.swsCtx)
		if swsNew != nil {
			r.swsCtx = swsNew
		}
		if err != nil {
			return nil, err
		}
		if len(jpeg) > 0 {
			return jpeg, nil
		}
	}
}

// decodeFrameToJPEG converts a decoded frame to JPEG bytes using the existing
// RGBA → JPEG pipeline. It reuses swsCtx when the frame dimensions and pixel
// format match; otherwise it creates a new SoftwareScaleContext.
func decodeFrameToJPEG(frame *astiav.Frame, swsCtx *astiav.SoftwareScaleContext) ([]byte, *astiav.SoftwareScaleContext, error) {
	w, h := frame.Width(), frame.Height()
	if w == 0 || h == 0 {
		return nil, nil, nil
	}

	srcFormat := frame.PixelFormat()
	if srcFormat == astiav.PixelFormatNone {
		return nil, nil, fmt.Errorf("frame has no pixel format")
	}

	created := false
	if swsCtx == nil ||
		swsCtx.SourceWidth() != w ||
		swsCtx.SourceHeight() != h ||
		swsCtx.SourcePixelFormat() != srcFormat ||
		swsCtx.DestinationWidth() != w ||
		swsCtx.DestinationHeight() != h ||
		swsCtx.DestinationPixelFormat() != astiav.PixelFormatRgba {
		if swsCtx != nil {
			swsCtx.Free()
		}
		var err error
		swsCtx, err = astiav.CreateSoftwareScaleContext(
			w, h, srcFormat,
			w, h, astiav.PixelFormatRgba,
			astiav.NewSoftwareScaleContextFlags(astiav.SoftwareScaleContextFlagFastBilinear),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("create sws context: %w", err)
		}
		created = true
	}

	rgbaFrame := astiav.AllocFrame()
	if rgbaFrame == nil {
		return nil, swsCtx, fmt.Errorf("allocate RGBA frame")
	}
	defer func() {
		if rgbaFrame != nil {
			rgbaFrame.Free()
		}
	}()
	rgbaFrame.SetWidth(w)
	rgbaFrame.SetHeight(h)
	rgbaFrame.SetPixelFormat(astiav.PixelFormatRgba)
	if err := rgbaFrame.AllocBuffer(1); err != nil {
		return nil, nil, fmt.Errorf("alloc rgba buffer: %w", err)
	}

	if err := swsCtx.ScaleFrame(frame, rgbaFrame); err != nil {
		return nil, nil, fmt.Errorf("scale frame: %w", err)
	}

	size, err := rgbaFrame.ImageBufferSize(1)
	if err != nil {
		return nil, nil, fmt.Errorf("image buffer size: %w", err)
	}
	raw := make([]byte, size)
	if _, err := rgbaFrame.ImageCopyToBuffer(raw, 1); err != nil {
		return nil, nil, fmt.Errorf("image copy to buffer: %w", err)
	}

	img := &image.NRGBA{
		Pix:    raw,
		Stride: w * 4,
		Rect:   image.Rect(0, 0, w, h),
	}

	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 70}); err != nil {
		return nil, nil, fmt.Errorf("jpeg encode: %w", err)
	}

	if created {
		return jpegBuf.Bytes(), swsCtx, nil
	}
	return jpegBuf.Bytes(), nil, nil
}

// emitError maps low-level FFmpeg/AVFoundation errors to user-facing messages,
// emits a usb:error event, and cancels the relay.
func (r *USBCameraRelay) emitError(err error, emitEvent func(name string, data any)) {
	if errors.Is(r.ctx.Err(), context.Canceled) {
		return
	}
	log.Printf("usb camera emitError (source=%s): %v", r.sourceID, err)
	msg := err.Error()
	switch {
	case errors.Is(err, astiav.ErrEperm),
		errors.Is(err, astiav.ErrEio),
		strings.Contains(strings.ToLower(msg), "denied"),
		strings.Contains(strings.ToLower(msg), "permission"):
		msg = "Camera permission was denied. Grant camera access to Lumivue in System Settings."
	case errors.Is(err, astiav.ErrEnoent),
		strings.Contains(strings.ToLower(msg), "not found"):
		msg = "Camera was disconnected or not found."
	}
	emitEvent("usb:error", map[string]any{"sourceId": r.sourceID, "error": msg})
	r.cancel()
}
