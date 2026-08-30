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
	"sync"
	"time"

	"github.com/asticode/go-astiav"
	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
	"github.com/pion/rtp"
)

// H264 Annex-B start code.
var annexBStartCode = []byte{0x00, 0x00, 0x00, 0x01}

// RTSPRelay connects to an RTSP stream, decodes H.264 frames, and serves them
// as an MJPEG stream over a local HTTP server.
type RTSPRelay struct {
	sourceID string
	url      string

	ctx    context.Context
	cancel context.CancelFunc

	port   int
	server *http.Server

	// subscriber broadcast
	mu          sync.Mutex
	subscribers map[chan []byte]struct{}
}

// newRTSPRelay allocates a relay without starting it.
func newRTSPRelay(sourceID, url string) *RTSPRelay {
	ctx, cancel := context.WithCancel(context.Background())
	return &RTSPRelay{
		sourceID:    sourceID,
		url:         url,
		ctx:         ctx,
		cancel:      cancel,
		subscribers: make(map[chan []byte]struct{}),
	}
}

// Start launches the RTSP connection, decoder, and MJPEG HTTP server.
// Returns the localhost port on which the MJPEG stream is served.
func (r *RTSPRelay) Start(emitEvent func(name string, data any)) (int, error) {
	// Pick a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listen: %w", err)
	}
	r.port = ln.Addr().(*net.TCPAddr).Port

	// Start the HTTP MJPEG server.
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", r.serveMJPEG)
	r.server = &http.Server{Handler: mux}
	go func() {
		if err := r.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("rtsp relay http server error (source=%s): %v", r.sourceID, err)
		}
	}()

	// Start the RTSP read loop in a goroutine.
	go r.readLoop(emitEvent)

	return r.port, nil
}

// Stop cancels the relay context, closing the RTSP connection and HTTP server.
func (r *RTSPRelay) Stop() {
	r.cancel()
	if r.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = r.server.Shutdown(ctx)
	}
}

// subscribe registers a new MJPEG client and returns a frame channel and unsubscribe func.
func (r *RTSPRelay) subscribe() (<-chan []byte, func()) {
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

// broadcast sends the latest JPEG frame to all subscribed clients (dropping if slow).
func (r *RTSPRelay) broadcast(frame []byte) {
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
func (r *RTSPRelay) serveMJPEG(w http.ResponseWriter, req *http.Request) {
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

// readLoop connects to the RTSP source, reads H.264 frames, decodes them to
// JPEG, and broadcasts. On a clean disconnect it waits 3 s then reconnects.
// It stops when the relay context is cancelled or a hard connection error occurs.
func (r *RTSPRelay) readLoop(emitEvent func(name string, data any)) {
	for {
		if err := r.connect(emitEvent); err != nil {
			if errors.Is(r.ctx.Err(), context.Canceled) {
				return
			}
			log.Printf("rtsp relay connect error (source=%s): %v", r.sourceID, err)
			emitEvent("rtsp:error", map[string]any{"sourceId": r.sourceID, "error": err.Error()})
			return
		}

		// connect returned nil: either the context was cancelled or the stream
		// ended cleanly. If cancelled, stop; otherwise wait and reconnect.
		if errors.Is(r.ctx.Err(), context.Canceled) {
			return
		}

		emitEvent("rtsp:reconnecting", map[string]any{"sourceId": r.sourceID})
		select {
		case <-time.After(3 * time.Second):
		case <-r.ctx.Done():
			return
		}
	}
}

// connect establishes one RTSP session and decodes frames until disconnected or
// the context is cancelled. Returns non-nil error on connection failure.
func (r *RTSPRelay) connect(emitEvent func(name string, data any)) error {
	u, err := base.ParseURL(r.url)
	if err != nil {
		return fmt.Errorf("parse rtsp url: %w", err)
	}

	c := &gortsplib.Client{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// Apply 5-second connection timeout.
	connectCtx, connectCancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer connectCancel()

	connectDone := make(chan error, 1)
	go func() {
		connectDone <- c.Start(u.Scheme, u.Host)
	}()

	select {
	case err := <-connectDone:
		if err != nil {
			return fmt.Errorf("rtsp connect: %w", err)
		}
	case <-connectCtx.Done():
		return fmt.Errorf("rtsp connect timeout")
	}
	defer c.Close()

	desc, _, err := c.Describe(u)
	if err != nil {
		return fmt.Errorf("rtsp describe: %w", err)
	}

	var forma *format.H264
	medi := desc.FindFormat(&forma)
	if medi == nil {
		return fmt.Errorf("no H264 track found in RTSP stream")
	}

	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		return fmt.Errorf("create rtp decoder: %w", err)
	}

	if _, err := c.Setup(desc.BaseURL, medi, 0, 0); err != nil {
		return fmt.Errorf("rtsp setup: %w", err)
	}

	// Initialise go-astiav H.264 decoder.
	codec := astiav.FindDecoder(astiav.CodecIDH264)
	if codec == nil {
		return fmt.Errorf("H264 decoder not found in FFmpeg")
	}
	decoderCtx := astiav.AllocCodecContext(codec)
	if err := decoderCtx.Open(codec, nil); err != nil {
		return fmt.Errorf("open h264 codec: %w", err)
	}
	defer decoderCtx.Free()

	var swsCtx *astiav.SoftwareScaleContext
	// Free the scale context whenever connect returns (ctx cancel or stream end).
	defer func() {
		if swsCtx != nil {
			swsCtx.Free()
		}
	}()

	// RTP → NALUs → JPEG pipeline.
	c.OnPacketRTP(medi, forma, func(pkt *rtp.Packet) {
		au, err := rtpDec.Decode(pkt)
		if err != nil {
			if !errors.Is(err, rtph264.ErrMorePacketsNeeded) &&
				!errors.Is(err, rtph264.ErrNonStartingPacketAndNoPrevious) {
				log.Printf("rtp decode error (source=%s): %v", r.sourceID, err)
			}
			return
		}

		jpegData, swsNew, err := decodeAccessUnit(au, forma.SPS, forma.PPS, decoderCtx, swsCtx)
		if err != nil {
			log.Printf("h264 decode error (source=%s): %v", r.sourceID, err)
			return
		}
		if swsNew != nil {
			swsCtx = swsNew
		}
		if jpegData != nil {
			r.broadcast(jpegData)
		}
	})

	if _, err := c.Play(nil); err != nil {
		return fmt.Errorf("rtsp play: %w", err)
	}

	// Block until the context is cancelled or the RTSP stream ends.
	// Any stream termination (server close, EOF, error) is treated as a clean
	// disconnect so that readLoop can schedule a reconnect attempt.
	waitDone := make(chan error, 1)
	go func() { waitDone <- c.Wait() }()

	select {
	case <-r.ctx.Done():
		return nil
	case <-waitDone:
		// Stream ended (normally or abnormally); return nil so readLoop can
		// decide whether to reconnect.
		return nil
	}
}

// decodeAccessUnit decodes a single H.264 access unit into a JPEG byte slice.
// It returns the updated SoftwareScaleContext if it was (re-)created.
func decodeAccessUnit(
	au [][]byte,
	sps, pps []byte,
	decoderCtx *astiav.CodecContext,
	swsCtx *astiav.SoftwareScaleContext,
) (jpegData []byte, newSwsCtx *astiav.SoftwareScaleContext, err error) {
	// Build Annex-B stream.
	// If the access unit already carries in-band SPS/PPS (NALU type 7/8), use
	// them as-is; otherwise prepend the out-of-band SPS/PPS from the format
	// description.  Prepending when in-band copies are present causes duplicate
	// parameter sets that confuse the decoder.
	hasParameterSets := false
	for _, nalu := range au {
		if len(nalu) > 0 {
			switch nalu[0] & 0x1F {
			case 7, 8: // SPS, PPS
				hasParameterSets = true
			}
		}
	}

	var buf bytes.Buffer
	if !hasParameterSets {
		if len(sps) > 0 {
			buf.Write(annexBStartCode)
			buf.Write(sps)
		}
		if len(pps) > 0 {
			buf.Write(annexBStartCode)
			buf.Write(pps)
		}
	}
	for _, nalu := range au {
		buf.Write(annexBStartCode)
		buf.Write(nalu)
	}

	pkt := astiav.AllocPacket()
	defer pkt.Free()
	if err := pkt.FromData(buf.Bytes()); err != nil {
		return nil, nil, fmt.Errorf("packet from data: %w", err)
	}

	if err := decoderCtx.SendPacket(pkt); err != nil {
		return nil, nil, fmt.Errorf("send packet: %w", err)
	}

	frame := astiav.AllocFrame()
	defer frame.Free()

	if err := decoderCtx.ReceiveFrame(frame); err != nil {
		if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("receive frame: %w", err)
	}

	w, h := frame.Width(), frame.Height()
	if w == 0 || h == 0 {
		return nil, nil, nil
	}

	// Create or reuse the pixel-format converter (YUV → RGBA).
	if swsCtx == nil {
		swsCtx, err = astiav.CreateSoftwareScaleContext(
			w, h, frame.PixelFormat(),
			w, h, astiav.PixelFormatRgba,
			astiav.NewSoftwareScaleContextFlags(astiav.SoftwareScaleContextFlagFastBilinear),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("create sws context: %w", err)
		}
		newSwsCtx = swsCtx
	}

	rgbaFrame := astiav.AllocFrame()
	defer rgbaFrame.Free()
	rgbaFrame.SetWidth(w)
	rgbaFrame.SetHeight(h)
	rgbaFrame.SetPixelFormat(astiav.PixelFormatRgba)
	if err := rgbaFrame.AllocBuffer(1); err != nil {
		return nil, newSwsCtx, fmt.Errorf("alloc rgba buffer: %w", err)
	}

	if err := swsCtx.ScaleFrame(frame, rgbaFrame); err != nil {
		return nil, newSwsCtx, fmt.Errorf("scale frame: %w", err)
	}

	size, err := rgbaFrame.ImageBufferSize(1)
	if err != nil {
		return nil, newSwsCtx, fmt.Errorf("image buffer size: %w", err)
	}
	raw := make([]byte, size)
	if _, err := rgbaFrame.ImageCopyToBuffer(raw, 1); err != nil {
		return nil, newSwsCtx, fmt.Errorf("image copy to buffer: %w", err)
	}

	img := &image.NRGBA{
		Pix:    raw,
		Stride: w * 4,
		Rect:   image.Rect(0, 0, w, h),
	}

	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 70}); err != nil {
		return nil, newSwsCtx, fmt.Errorf("jpeg encode: %w", err)
	}

	return jpegBuf.Bytes(), newSwsCtx, nil
}
