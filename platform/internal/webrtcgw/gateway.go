package webrtcgw

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/emiago/diago"
	"github.com/pion/webrtc/v4"
)

type Gateway struct {
	log   *slog.Logger
	api   *webrtc.API
	cfg   webrtc.Configuration
	mu    sync.Mutex
	peers map[string]*webrtc.PeerConnection
	// TrackHandler is invoked when a new inbound track is received.
	// It allows the platform to wire the incoming track to other media subsystems (eg. diago).
	TrackHandler func(pcID string, track *webrtc.TrackRemote, pc *webrtc.PeerConnection)
	// mediaMap stores DialogMedia created from WebRTC peers keyed by pcID
	mediaMap map[string]*diago.DialogMedia
}

func NewGateway(log *slog.Logger) *Gateway {
	return &Gateway{
		log:      log,
		api:      webrtc.NewAPI(),
		peers:    map[string]*webrtc.PeerConnection{},
		mediaMap: map[string]*diago.DialogMedia{},
	}
}

// StoreDialogMedia stores a DialogMedia for a peer connection id
func (g *Gateway) StoreDialogMedia(pcID string, dm *diago.DialogMedia) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if pcID == "" {
		// fallback to local addr string if missing
		if ms := dm.MediaSession(); ms != nil {
			pcID = ms.Laddr.String()
		}
	}
	g.mediaMap[pcID] = dm
}

// GetDialogMedia returns stored DialogMedia by pcID
func (g *Gateway) GetDialogMedia(pcID string) (*diago.DialogMedia, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	dm, ok := g.mediaMap[pcID]
	return dm, ok
}

// SetTrackHandler sets callback invoked for each inbound track.
func (g *Gateway) SetTrackHandler(h func(pcID string, track *webrtc.TrackRemote, pc *webrtc.PeerConnection)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.TrackHandler = h
}

type OfferRequest struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}

type OfferResponse struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}

func (g *Gateway) HandleOffer(ctx context.Context, req OfferRequest) (OfferResponse, error) {
	if req.SDP == "" {
		return OfferResponse{}, fmt.Errorf("offer sdp is empty")
	}

	pc, err := g.api.NewPeerConnection(g.cfg)
	if err != nil {
		return OfferResponse{}, fmt.Errorf("new peer connection: %w", err)
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		g.log.Info("webrtc state", "state", state.String())
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			_ = pc.Close()
		}
	})

	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		g.log.Info("track received", "kind", track.Kind().String(), "codec", track.Codec().MimeType)
		// If platform provided a TrackHandler, hand off the track for bridging
		if g.TrackHandler != nil {
			// use local copy of pc's current local description as id
			pcID := ""
			if pc.LocalDescription() != nil {
				pcID = pc.LocalDescription().SDP
			}
			go g.TrackHandler(pcID, track, pc)
			return
		}

		// Default behavior: drain track packets (avoid blocking the PeerConnection)
		go func() {
			buf := make([]byte, 1500)
			for {
				_, _, readErr := track.Read(buf)
				if readErr != nil {
					g.log.Debug("track read closed", "err", readErr)
					return
				}
			}
		}()
	})

	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: req.SDP}
	if err := pc.SetRemoteDescription(offer); err != nil {
		_ = pc.Close()
		return OfferResponse{}, fmt.Errorf("set remote description: %w", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		_ = pc.Close()
		return OfferResponse{}, fmt.Errorf("create answer: %w", err)
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		_ = pc.Close()
		return OfferResponse{}, fmt.Errorf("set local description: %w", err)
	}

	<-webrtc.GatheringCompletePromise(pc)
	local := pc.LocalDescription()
	if local == nil {
		_ = pc.Close()
		return OfferResponse{}, fmt.Errorf("local description is nil")
	}

	g.mu.Lock()
	g.peers[local.SDP] = pc
	g.mu.Unlock()

	return OfferResponse{SDP: local.SDP, Type: local.Type.String()}, nil
}
