package ai

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"
)

type Gateway struct {
	log *slog.Logger

	mu sync.Mutex
	// asrSessions keyed by pcID
	asrSessions map[string]*ASRSession
}

func NewGateway(log *slog.Logger) *Gateway {
	return &Gateway{log: log, asrSessions: map[string]*ASRSession{}}
}

type ASRResult struct {
	Text   string `json:"text"`
	Final  bool   `json:"final"`
	Lang   string `json:"lang,omitempty"`
	Offset int64  `json:"offset,omitempty"`
}

type ASRSession struct {
	pcID    string
	cancel  context.CancelFunc
	results chan ASRResult
}

// StartASR starts a fake ASR session that reads from provided reader and emits periodic results.
func (g *Gateway) StartASR(pcID string, r io.Reader) (<-chan ASRResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.asrSessions[pcID]; ok {
		return nil, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &ASRSession{pcID: pcID, cancel: cancel, results: make(chan ASRResult, 8)}
	g.asrSessions[pcID] = s

	go func() {
		defer close(s.results)
		buf := make([]byte, 4096)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		offset := int64(0)
		for i := 0; i < 30; i++ { // limit to avoid runaway in stub
			select {
			case <-ctx.Done():
				s.results <- ASRResult{Text: "", Final: true, Offset: offset}
				return
			case <-ticker.C:
				// attempt to read some audio bytes to advance offset
				if n, _ := r.Read(buf); n > 0 {
					offset += int64(n)
				}
				// emit a fake partial result
				s.results <- ASRResult{Text: "(partial) hello world", Final: false, Offset: offset}
			}
		}
		// after loop emit final
		s.results <- ASRResult{Text: "hello world", Final: true, Offset: offset}
	}()

	return s.results, nil
}

func (g *Gateway) StopASR(pcID string) {
	g.mu.Lock()
	s, ok := g.asrSessions[pcID]
	if ok {
		delete(g.asrSessions, pcID)
	}
	g.mu.Unlock()
	if ok && s.cancel != nil {
		s.cancel()
	}
}

func (g *Gateway) Health() map[string]string {
	return map[string]string{"status": "ready", "provider": "stub"}
}
