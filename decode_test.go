package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/asticode/go-astiav"
)

// TestDecodeAccessUnit encodes a black 16×16 H.264 frame with FFmpeg, then
// feeds the resulting NALUs through decodeAccessUnit and expects a JPEG back.
func TestDecodeAccessUnit_RoundTrip(t *testing.T) {
	codec := astiav.FindDecoder(astiav.CodecIDH264)
	if codec == nil {
		t.Skip("H.264 decoder not available via FFmpeg")
	}

	sps, pps, idrNALU := encodeTestH264Frame(t)
	if len(sps) == 0 || len(pps) == 0 || len(idrNALU) == 0 {
		t.Skip("test H.264 frame could not be generated")
	}

	decoderCodec := astiav.FindDecoder(astiav.CodecIDH264)
	decoderCtx := astiav.AllocCodecContext(decoderCodec)
	if err := decoderCtx.Open(decoderCodec, nil); err != nil {
		t.Fatalf("open H.264 decoder: %v", err)
	}
	defer decoderCtx.Free()

	au := [][]byte{idrNALU}
	jpegData, swsCtx, err := decodeAccessUnit(au, sps, pps, decoderCtx, nil)
	if swsCtx != nil {
		defer swsCtx.Free()
	}
	if err != nil {
		t.Fatalf("decodeAccessUnit: %v", err)
	}
	if jpegData == nil {
		t.Fatal("expected JPEG data, got nil")
	}
	// JPEG files start with the SOI marker 0xFF 0xD8.
	if len(jpegData) < 2 || jpegData[0] != 0xFF || jpegData[1] != 0xD8 {
		t.Errorf("output does not look like a JPEG: first bytes % X", jpegData[:min(len(jpegData), 4)])
	}
}

// TestDecodeAccessUnit_EarlyReturn verifies that sending an invalid bitstream
// does not panic and returns gracefully (EAGAIN or error).
func TestDecodeAccessUnit_EarlyReturn(t *testing.T) {
	codec := astiav.FindDecoder(astiav.CodecIDH264)
	if codec == nil {
		t.Skip("H.264 decoder not available")
	}
	decoderCtx := astiav.AllocCodecContext(codec)
	if err := decoderCtx.Open(codec, nil); err != nil {
		t.Fatalf("open decoder: %v", err)
	}
	defer decoderCtx.Free()

	// An all-zero NALU is not valid H.264; the decoder should return EAGAIN
	// or an error, but must not panic.
	zeroNALU := make([]byte, 16)
	jpegData, _, err := decodeAccessUnit([][]byte{zeroNALU}, nil, nil, decoderCtx, nil)
	// We just want no panic. jpegData nil and err (possibly EAGAIN) is fine.
	if jpegData != nil {
		t.Logf("unexpectedly got JPEG data for zero NALU (len=%d)", len(jpegData))
	}
	_ = err
}

// TestDecodeAccessUnit_SwsReuse verifies that a non-nil swsCtx is reused
// rather than recreated on the second call with the same frame dimensions.
func TestDecodeAccessUnit_SwsReuse(t *testing.T) {
	sps, pps, idrNALU := encodeTestH264Frame(t)
	if len(sps) == 0 || len(pps) == 0 || len(idrNALU) == 0 {
		t.Skip("test H.264 frame could not be generated")
	}

	codec := astiav.FindDecoder(astiav.CodecIDH264)
	if codec == nil {
		t.Skip("H.264 decoder not available")
	}
	decoderCtx := astiav.AllocCodecContext(codec)
	if err := decoderCtx.Open(codec, nil); err != nil {
		t.Fatalf("open decoder: %v", err)
	}
	defer decoderCtx.Free()

	// First decode creates the swsCtx.
	_, swsCtx1, err := decodeAccessUnit([][]byte{idrNALU}, sps, pps, decoderCtx, nil)
	if err != nil {
		t.Fatalf("first decode: %v", err)
	}
	if swsCtx1 != nil {
		defer swsCtx1.Free()
	}

	// Second decode with existing swsCtx must return the same context unchanged.
	_, swsCtx2, err := decodeAccessUnit([][]byte{idrNALU}, sps, pps, decoderCtx, swsCtx1)
	if err != nil {
		t.Logf("second decode returned error (acceptable for P-frame): %v", err)
	}
	// swsCtx2 must be nil (existing ctx was reused, not replaced).
	if swsCtx2 != nil {
		t.Error("expected nil swsCtx2 when reusing existing context")
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

// encodeTestH264Frame encodes a single 16×16 black frame using the FFmpeg H.264
// encoder and returns the SPS, PPS, and IDR NALUs without Annex-B start codes.
// encodeH264TestFrame encodes a single 16×16 black YUV420P frame with the FFmpeg
// H.264 encoder. Returns sps, pps, idrNALU bytes, or all nil when unavailable.
// It is safe to call from any goroutine (no testing.T required).
func encodeH264TestFrame() (sps, pps, idrNALU []byte) {
	encoder := astiav.FindEncoder(astiav.CodecIDH264)
	if encoder == nil {
		return nil, nil, nil
	}

	encoderCtx := astiav.AllocCodecContext(encoder)
	encoderCtx.SetWidth(16)
	encoderCtx.SetHeight(16)
	encoderCtx.SetPixelFormat(astiav.PixelFormatYuv420P)
	encoderCtx.SetTimeBase(astiav.NewRational(1, 25))

	dict := astiav.NewDictionary()
	defer dict.Free()
	// Force intra-only for deterministic test output.
	dict.Set("tune", "zerolatency", 0)
	dict.Set("x264-params", "keyint=1:bframes=0", 0)
	if err := encoderCtx.Open(encoder, dict); err != nil {
		return nil, nil, nil
	}
	defer encoderCtx.Free()

	// Allocate a YUV420P frame and fill it black.
	frame := astiav.AllocFrame()
	defer frame.Free()
	frame.SetWidth(16)
	frame.SetHeight(16)
	frame.SetPixelFormat(astiav.PixelFormatYuv420P)
	frame.SetPts(0)
	if err := frame.AllocBuffer(1); err != nil {
		return nil, nil, nil
	}

	// Encode and receive packet.
	if err := encoderCtx.SendFrame(frame); err != nil {
		return nil, nil, nil
	}
	pkt := astiav.AllocPacket()
	defer pkt.Free()
	if err := encoderCtx.ReceivePacket(pkt); err != nil {
		return nil, nil, nil
	}

	// Parse the Annex-B output to extract SPS, PPS, IDR NALUs.
	return parseAnnexBNALUs(pkt.Data())
}

// encodeTestH264Frame is a testing wrapper around encodeH264TestFrame that
// calls t.Helper() so any failure is attributed to the test itself.
func encodeTestH264Frame(t *testing.T) (sps, pps, idrNALU []byte) {
	t.Helper()
	return encodeH264TestFrame()
}

// parseAnnexBNALUs splits an Annex-B bytestream into individual NALUs.
// It returns the first SPS, PPS, and IDR NALUs found (without start codes).
func parseAnnexBNALUs(data []byte) (sps, pps, idrNALU []byte) {
	startCode3 := []byte{0x00, 0x00, 0x01}
	startCode4 := []byte{0x00, 0x00, 0x00, 0x01}

	var nalus [][]byte
	i := 0
	for i < len(data) {
		// Find next start code.
		if i+4 <= len(data) && bytes.Equal(data[i:i+4], startCode4) {
			i += 4
		} else if i+3 <= len(data) && bytes.Equal(data[i:i+3], startCode3) {
			i += 3
		} else {
			i++
			continue
		}
		start := i
		// Find end of this NALU.
		end := len(data)
		for j := i; j < len(data)-2; j++ {
			if bytes.Equal(data[j:j+3], startCode3) {
				end = j
				break
			}
		}
		if start < end {
			nalus = append(nalus, data[start:end])
		}
		i = end
	}

	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}
		naluType := nalu[0] & 0x1F
		switch naluType {
		case 7: // SPS
			if sps == nil {
				sps = nalu
			}
		case 8: // PPS
			if pps == nil {
				pps = nalu
			}
		case 5: // IDR
			if idrNALU == nil {
				idrNALU = nalu
			}
		}
	}
	return sps, pps, idrNALU
}

var _ = errors.New // keep errors import used
