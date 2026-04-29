package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"github.com/emiago/ai-call-center-platform/internal/ai"
	"github.com/emiago/ai-call-center-platform/internal/call"
	"github.com/emiago/ai-call-center-platform/internal/ivr"
	"github.com/emiago/ai-call-center-platform/internal/webrtcgw"
	"github.com/emiago/diago"
	"github.com/emiago/sipgo/sip"
)

type Deps struct {
	Logger        *slog.Logger
	CallService   *call.Service
	AIGateway     *ai.Gateway
	WebRTCGateway *webrtcgw.Gateway
	Diago         *diago.Diago
	IVREngine     *ivr.Engine
	CorsOrigins   []string
}

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("GET /v1/ai/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.AIGateway.Health())
	})

	mux.HandleFunc("GET /v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		sessions, err := deps.CallService.ListSessions(r.Context(), 20)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, sessions)
	})

	mux.HandleFunc("POST /v1/webrtc/offer", func(w http.ResponseWriter, r *http.Request) {
		var req webrtcgw.OfferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		res, err := deps.WebRTCGateway.HandleOffer(r.Context(), req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("POST /v1/webrtc/attach", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PCID      string `json:"pcId"`
			Recipient string `json:"recipient"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}

		dm, ok := deps.WebRTCGateway.GetDialogMedia(body.PCID)
		if !ok || dm == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "dialog media not found"})
			return
		}

		// Parse recipient into sip.Uri
		recipient := sip.Uri{}
		if err := sip.ParseUri(body.Recipient, &recipient); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid recipient uri"})
			return
		}

		// Create new dialog client and attach media
		d, err := deps.Diago.NewDialog(recipient, diago.NewDialogOptions{})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		// Attach media from stored DialogMedia
		if dm.MediaSession() != nil {
			d.InitMediaSession(dm.MediaSession(), dm.RTPPacketReader, dm.RTPPacketWriter)
		}

		// Persist session record in DB (best-effort)
		if deps.CallService != nil {
			sess := call.Session{
				CallID:    d.Id(),
				Direction: "outbound",
				State:     "inviting",
				TenantID:  1,
				StartedAt: time.Now(),
			}
			if err := deps.CallService.CreateSession(r.Context(), &sess); err != nil {
				deps.Logger.Warn("failed to persist session", "error", err)
			}
		}

		// Send invite in background
		go func() {
			_ = d.Invite(context.Background(), diago.InviteClientOptions{})
		}()

		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "dialogId": d.Id()})
	})

	// IVR / ACD endpoints
	mux.HandleFunc("POST /v1/ivr/enqueue", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			SessionID uint `json:"session_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s, err := deps.CallService.GetSessionByID(r.Context(), body.SessionID)
		if err != nil {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		deps.IVREngine.Enqueue(*s)
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("POST /v1/agents/register", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AgentID string `json:"agent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		deps.IVREngine.RegisterAgent(body.AgentID)
		w.WriteHeader(http.StatusCreated)
	})

	mux.HandleFunc("GET /v1/agents", func(w http.ResponseWriter, r *http.Request) {
		agents := deps.IVREngine.ListAgents()
		_ = json.NewEncoder(w).Encode(agents)
	})

	// WebSocket endpoint for ASR real-time results
	mux.HandleFunc("GET /v1/ws/asr", func(w http.ResponseWriter, r *http.Request) {
		pcid := r.URL.Query().Get("pcid")
		if pcid == "" {
			http.Error(w, "missing pcid", http.StatusBadRequest)
			return
		}

		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			deps.Logger.Error("ws upgrade", "err", err)
			return
		}
		defer conn.Close()

		ch, unsub := deps.AIGateway.SubscribeASR(pcid)
		defer unsub()

		// send existing ASR messages until client disconnects
		for res := range ch {
			b, _ := json.Marshal(res)
			if err := wsutil.WriteServerMessage(conn, ws.OpText, b); err != nil {
				break
			}
			if res.Final {
				// after final, continue to wait for possible future sessions
			}
		}
	})

	mux.HandleFunc("POST /v1/webrtc/asr/start", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PCID string `json:"pcId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}

		dm, ok := deps.WebRTCGateway.GetDialogMedia(body.PCID)
		if !ok || dm == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "dialog media not found"})
			return
		}

		// get audio reader
		rdr, err := dm.AudioReader()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		ch, err := deps.AIGateway.StartASR(body.PCID, rdr)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		// consume one partial result to ensure started
		go func() {
			for res := range ch {
				deps.Logger.Info("ASR result", "pcid", body.PCID, "text", res.Text, "final", res.Final)
			}
		}()

		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
	})

	mux.HandleFunc("POST /v1/webrtc/asr/stop", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PCID string `json:"pcId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		deps.AIGateway.StopASR(body.PCID)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	return withCORS(mux, deps.CorsOrigins)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func withCORS(next http.Handler, origins []string) http.Handler {
	allowed := map[string]struct{}{}
	allowAll := false
	for _, origin := range origins {
		if origin == "*" {
			allowAll = true
			break
		}
		allowed[origin] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowAll {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		} else if origin != "" {
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}
		if origin != "" {
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
