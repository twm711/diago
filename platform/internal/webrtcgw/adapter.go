package webrtcgw

import (
	"io"
	"log/slog"

	"github.com/emiago/diago/media"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// PionRTPReader implements media.RTPReader by reading parsed RTP packets from a Pion TrackRemote
type PionRTPReader struct {
	track *webrtc.TrackRemote
}

func NewPionRTPReader(t *webrtc.TrackRemote) *PionRTPReader {
	return &PionRTPReader{track: t}
}

// ReadRTP reads a packet from the remote track and fills provided rtp.Packet.
func (r *PionRTPReader) ReadRTP(buf []byte, p *rtp.Packet) (int, error) {
	pkt, _, err := r.track.ReadRTP()
	if err != nil {
		return 0, err
	}
	// copy packet data into provided packet structure
	*p = *pkt
	// compute raw size (header + payload + padding)
	n := p.Header.MarshalSize() + len(p.Payload) + int(p.PaddingSize)
	return n, nil
}

// PionRTPWriter implements media.RTPWriter by writing RTP packets to a Pion local track
type PionRTPWriter struct {
	log   *slog.Logger
	track *webrtc.TrackLocalStaticRTP
}

func NewPionRTPWriter(log *slog.Logger, t *webrtc.TrackLocalStaticRTP) *PionRTPWriter {
	return &PionRTPWriter{log: log, track: t}
}

func (w *PionRTPWriter) WriteRTP(p *rtp.Packet) error {
	// write packet to local track
	return w.track.WriteRTP(p)
}

// Helper: connect reader -> writer using media RTP packet helpers
func BridgeTrackToTrack(log *slog.Logger, incoming *webrtc.TrackRemote, outgoing *webrtc.TrackLocalStaticRTP) error {
	// map payload type to media codec
	codec, _ := media.CodecAudioFromPayloadType(uint8(incoming.PayloadType()))

	reader := NewPionRTPReader(incoming)
	rtpReader := media.NewRTPPacketReader(reader, codec)

	writer := NewPionRTPWriter(log, outgoing)
	rtpWriter := media.NewRTPPacketWriter(writer, codec)

	// copy payload frames from reader to writer
	go func() {
		_, _ = io.Copy(rtpWriter, rtpReader)
	}()
	return nil
}

// CreateBridgeForIncoming creates a local track and RTP packet reader/writer adapters
// for an incoming remote track. It starts a goroutine that forwards incoming RTP
// packets into the returned writer which writes to the created local track.
func CreateBridgeForIncoming(log *slog.Logger, incoming *webrtc.TrackRemote) (
	*media.RTPPacketReader, *media.RTPPacketWriter, *webrtc.TrackLocalStaticRTP, error) {

	codec, _ := media.CodecAudioFromPayloadType(uint8(incoming.PayloadType()))

	reader := NewPionRTPReader(incoming)
	rtpReader := media.NewRTPPacketReader(reader, codec)

	localTrack, err := webrtc.NewTrackLocalStaticRTP(incoming.Codec().RTPCodecCapability, "audio", "pion")
	if err != nil {
		return nil, nil, nil, err
	}

	writer := NewPionRTPWriter(log, localTrack)
	rtpWriter := media.NewRTPPacketWriter(writer, codec)

	// Forward incoming -> outgoing track via packet reader/writer
	go func() {
		_, _ = io.Copy(rtpWriter, rtpReader)
	}()

	return rtpReader, rtpWriter, localTrack, nil
}
