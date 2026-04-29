package ai

import (
    "context"
    "io"
    "sync"
    "time"
)

// WSProvider is a scaffold for a WebSocket-based ASR provider.
// Current implementation is an in-process emulation (no network).
type WSProvider struct {
    Transcoder Transcoder
    mu         sync.Mutex
    sessions   map[string]context.CancelFunc
}

func (w *WSProvider) Start(ctx context.Context, pcID string, r io.Reader) (<-chan ASRResult, error) {
    if w.Transcoder == nil {
        w.Transcoder = &IdentityTranscoder{}
    }
    rdr, err := w.Transcoder.Transcode(r)
    if err != nil {
        return nil, err
    }

    ch := make(chan ASRResult, 8)
    cctx, cancel := context.WithCancel(ctx)
    w.mu.Lock()
    if w.sessions == nil {
        w.sessions = map[string]context.CancelFunc{}
    }
    w.sessions[pcID] = cancel
    w.mu.Unlock()

    go func() {
        defer close(ch)
        buf := make([]byte, 2048)
        ticker := time.NewTicker(1500 * time.Millisecond)
        defer ticker.Stop()
        offset := int64(0)
        for i := 0; i < 20; i++ {
            select {
            case <-cctx.Done():
                ch <- ASRResult{Text: "", Final: true, Offset: offset}
                return
            case <-ticker.C:
                if n, _ := rdr.Read(buf); n > 0 {
                    offset += int64(n)
                }
                // emulate provider partial + final results
                if i%5 == 0 {
                    ch <- ASRResult{Text: "(ws) partial result", Final: false, Offset: offset}
                } else {
                    ch <- ASRResult{Text: "(ws) continuing...", Final: false, Offset: offset}
                }
            }
        }
        ch <- ASRResult{Text: "(ws) final transcript", Final: true, Offset: offset}
    }()

    return ch, nil
}

func (w *WSProvider) Stop(pcID string) {
    w.mu.Lock()
    if w.sessions != nil {
        if cancel, ok := w.sessions[pcID]; ok {
            cancel()
            delete(w.sessions, pcID)
        }
    }
    w.mu.Unlock()
}
