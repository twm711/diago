package ai

import (
    "context"
    "io"
    "sync"
    "time"
)

// Provider is an abstraction for ASR providers. Implementations should consume audio
// from the provided Reader and return a channel of ASRResult. Implementations are
// responsible for decoding/transcoding audio as needed by the provider.
type Provider interface {
    Start(ctx context.Context, pcID string, r io.Reader) (<-chan ASRResult, error)
    Stop(pcID string)
}

// Transcoder is a pluggable audio transcoder. For real providers implement
// conversion (e.g., Opus -> PCM16). For now this is a placeholder identity
// transcoder used by the stub provider.
type Transcoder interface {
    Transcode(r io.Reader) (io.Reader, error)
}

// IdentityTranscoder returns the reader as-is.
type IdentityTranscoder struct{}

func (i *IdentityTranscoder) Transcode(r io.Reader) (io.Reader, error) { return r, nil }

// StubProvider is a simple in-process provider that emits fake ASR results.
type StubProvider struct {
    Transcoder Transcoder
    mu         sync.Mutex
    sessions   map[string]context.CancelFunc
}

func (s *StubProvider) Start(ctx context.Context, pcID string, r io.Reader) (<-chan ASRResult, error) {
    if s.Transcoder == nil {
        s.Transcoder = &IdentityTranscoder{}
    }
    rdr, err := s.Transcoder.Transcode(r)
    if err != nil {
        return nil, err
    }
    ch := make(chan ASRResult, 8)
    cctx, cancel := context.WithCancel(ctx)
    s.mu.Lock()
    if s.sessions == nil {
        s.sessions = map[string]context.CancelFunc{}
    }
    s.sessions[pcID] = cancel
    s.mu.Unlock()

    go func() {
        defer close(ch)
        buf := make([]byte, 4096)
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()
        offset := int64(0)
        for i := 0; i < 30; i++ {
            select {
            case <-cctx.Done():
                ch <- ASRResult{Text: "", Final: true, Offset: offset}
                return
            case <-ticker.C:
                if n, _ := rdr.Read(buf); n > 0 {
                    offset += int64(n)
                }
                ch <- ASRResult{Text: "(partial) hello world", Final: false, Offset: offset}
            }
        }
        ch <- ASRResult{Text: "hello world", Final: true, Offset: offset}
    }()

    return ch, nil
}

func (s *StubProvider) Stop(pcID string) {
    s.mu.Lock()
    if s.sessions != nil {
        if cancel, ok := s.sessions[pcID]; ok {
            cancel()
            delete(s.sessions, pcID)
        }
    }
    s.mu.Unlock()
}
