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
	// subscribers keyed by pcID -> set of chans
	subscribers map[string]map[chan ASRResult]struct{}
	// optional persist callback
	persistMu sync.Mutex
	persist   func(pcID string, res ASRResult)
}

func NewGateway(log *slog.Logger) *Gateway {
	return &Gateway{log: log, asrSessions: map[string]*ASRSession{}, subscribers: map[string]map[chan ASRResult]struct{}{}}
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
				res := ASRResult{Text: "", Final: true, Offset: offset}
				s.results <- res
				g.persistIfSet(pcID, res)
				g.broadcast(pcID, res)
				return
			case <-ticker.C:
				// attempt to read some audio bytes to advance offset
				if n, _ := r.Read(buf); n > 0 {
					offset += int64(n)
				}
				// emit a fake partial result
				res := ASRResult{Text: "(partial) hello world", Final: false, Offset: offset}
				s.results <- res
				g.persistIfSet(pcID, res)
				g.broadcast(pcID, res)
			}
		}
		// after loop emit final
		res := ASRResult{Text: "hello world", Final: true, Offset: offset}
		s.results <- res
		g.persistIfSet(pcID, res)
		g.broadcast(pcID, res)
	}()

	return s.results, nil
}

// SetPersist sets an optional callback invoked for every ASR result produced.
func (g *Gateway) SetPersist(fn func(pcID string, res ASRResult)) {
	g.persistMu.Lock()
	defer g.persistMu.Unlock()
	g.persist = fn
}

func (g *Gateway) persistIfSet(pcID string, res ASRResult) {
	g.persistMu.Lock()
	fn := g.persist
	g.persistMu.Unlock()
	if fn != nil {
		go fn(pcID, res)
	}
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

// SubscribeASR subscribes to ASR result stream for a pcID. Returns a channel and an unsubscribe func.
func (g *Gateway) SubscribeASR(pcID string) (<-chan ASRResult, func()) {
	ch := make(chan ASRResult, 8)
	g.mu.Lock()
	m, ok := g.subscribers[pcID]
	if !ok {
		m = map[chan ASRResult]struct{}{}
		g.subscribers[pcID] = m
	}
	m[ch] = struct{}{}
	g.mu.Unlock()
	unsub := func() {
		g.mu.Lock()
		if m, ok := g.subscribers[pcID]; ok {
			delete(m, ch)
			if len(m) == 0 {
				delete(g.subscribers, pcID)
			}
		}
		g.mu.Unlock()
		close(ch)
	}
	return ch, unsub
}

func (g *Gateway) broadcast(pcID string, res ASRResult) {
	g.mu.Lock()
	subs := g.subscribers[pcID]
	// make a snapshot to avoid holding lock while sending
	var chans []chan ASRResult
	for c := range subs {
		chans = append(chans, c)
	}
	g.mu.Unlock()
	for _, c := range chans {
		select {
		case c <- res:
		default:
		}
	}
}

func (g *Gateway) Health() map[string]string {
	return map[string]string{"status": "ready", "provider": "stub"}
}
