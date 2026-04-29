package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emiago/ai-call-center-platform/internal/ai"
	"github.com/emiago/ai-call-center-platform/internal/call"
	"github.com/emiago/ai-call-center-platform/internal/config"
	"github.com/emiago/ai-call-center-platform/internal/db"
	"github.com/emiago/ai-call-center-platform/internal/httpapi"
	"github.com/emiago/ai-call-center-platform/internal/ivr"
	"github.com/emiago/ai-call-center-platform/internal/webrtcgw"
	"github.com/emiago/diago"
	"github.com/emiago/diago/media"
	"github.com/emiago/sipgo"
	"github.com/pion/webrtc/v4"
	"net"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	database, err := db.Open(cfg.MySQLDSN)
	if err != nil {
		logger.Error("open mysql failed", "error", err)
		os.Exit(1)
	}

	callService := call.NewService(database, logger)
	aiGateway := ai.NewGateway(logger)
	// persist ASR results into DB
	aiGateway.SetPersist(func(pcID string, res ai.ASRResult) {
		// best-effort persistence: link by pcID -> session unknown here
		rec := call.ASRRecord{
			PCID:   pcID,
			Text:   res.Text,
			Final:  res.Final,
			Offset: res.Offset,
		}
		_ = callService.CreateASRRecord(context.Background(), &rec)
	})
	webrtcGateway := webrtcgw.NewGateway(logger)

	ivrEngine := ivr.NewEngine(logger)
	// simple assign handler: log assignment and mark session state via call service
	ivrEngine.SetAssignHandler(func(agentID string, s call.Session) {
		logger.Info("ivr: assigned", "agent", agentID, "call_id", s.CallID)
		// best-effort: update session state
		s.State = "assigned"
		_ = callService.UpdateSessionState(context.Background(), s.ID, s.State)
	})

	// Create diago core
	ua, _ := sipgo.NewUA()
	diagoCore := diago.NewDiago(ua)

	// Basic TrackHandler: bridge inbound audio into diago media layer and back to browser.
	webrtcGateway.SetTrackHandler(func(pcID string, track *webrtc.TrackRemote, pc *webrtc.PeerConnection) {
		logger.Info("webrtc: inbound track handler", "kind", track.Kind().String(), "codec", track.Codec().MimeType)
		if track.Kind() != webrtc.RTPCodecTypeAudio {
			return
		}

		// Create adapters and a local track
		rtpReader, rtpWriter, localTrack, err := webrtcgw.CreateBridgeForIncoming(logger, track)
		if err != nil {
			logger.Error("create bridge failed", "error", err)
			return
		}

		if _, err := pc.AddTrack(localTrack); err != nil {
			logger.Error("add local track failed", "error", err)
			return
		}

		// Initialize diago media session and attach packet reader/writer so the platform
		// can later connect this media to a Dialog (ACD/IVR/agent) lifecycle.
		sessIP := "127.0.0.1"
		msess, err := media.NewMediaSession(net.ParseIP(sessIP), 0)
		if err != nil {
			logger.Error("create media session failed", "error", err)
			return
		}

		dm := &diago.DialogMedia{}
		dm.InitMediaSession(msess, rtpReader, rtpWriter)

		// Store created media in gateway for later lookup/attach by call service
		webrtcGateway.StoreDialogMedia(pcID, dm)
	})

	router := httpapi.NewRouter(httpapi.Deps{
		Logger:        logger,
		CallService:   callService,
		AIGateway:     aiGateway,
		WebRTCGateway: webrtcGateway,
		Diago:         diagoCore,
		IVREngine:     ivrEngine,
		CorsOrigins:   cfg.CORSOrigins,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api server started", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-sigCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	if sqlDB, err := database.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
