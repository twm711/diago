package ai

import (
    "context"
    "encoding/json"
    "io"
    "log/slog"
    "net/http"
    "sync"
    "time"

    "github.com/gobwas/ws"
    "github.com/gobwas/ws/wsutil"
)

// WSNetProvider connects to an external ASR over WebSocket. It sends audio as
// binary frames and expects text or JSON frames with ASR results.
type WSNetProvider struct {
    URL     string
    Headers http.Header
    Transcoder Transcoder
    Logger *slog.Logger

    mu       sync.Mutex
    sessions map[string]context.CancelFunc
}

func NewWSNetProvider(url string, headers http.Header, logger *slog.Logger) *WSNetProvider {
    return &WSNetProvider{URL: url, Headers: headers, Logger: logger}
}

func (w *WSNetProvider) Start(ctx context.Context, pcID string, r io.Reader) (<-chan ASRResult, error) {
    if w.Transcoder == nil {
        w.Transcoder = &IdentityTranscoder{}
    }
    rdr, err := w.Transcoder.Transcode(r)
    if err != nil {
        return nil, err
    }

    ch := make(chan ASRResult, 16)
    cctx, cancel := context.WithCancel(ctx)
    w.mu.Lock()
    if w.sessions == nil {
        w.sessions = map[string]context.CancelFunc{}
    }
    w.sessions[pcID] = cancel
    w.mu.Unlock()

    // Dial websocket
    go func() {
        defer func() {
            w.mu.Lock()
            delete(w.sessions, pcID)
            w.mu.Unlock()
            close(ch)
        }()

        // connect
        d := ws.Dialer{}
        conn, _, _, err := d.Dial(context.Background(), w.URL)
        if err != nil {
            if w.Logger != nil {
                w.Logger.Error("ws dial failed", "err", err)
            }
            // propagate error via closed channel (no values)
            return
        }
        defer conn.Close()

        // writer: copy audio into binary ws frames
        go func() {
            buf := make([]byte, 2048)
            for {
                select {
                case <-cctx.Done():
                    return
                default:
                }
                n, err := rdr.Read(buf)
                if n > 0 {
                    _ = wsutil.WriteClientMessage(conn, ws.OpBinary, buf[:n])
                }
                if err != nil {
                    if err == io.EOF {
                        // signal end of stream, wait a bit then return
                        time.Sleep(200 * time.Millisecond)
                        return
                    }
                    return
                }
            }
        }()

        // reader: receive text/json frames and send as ASRResult
        for {
            if cctx.Err() != nil {
                return
            }
            msg, op, err := wsutil.ReadServerData(conn)
            if err != nil {
                return
            }
            if op == ws.OpText || op == ws.OpBinary {
                // try parse JSON
                var res ASRResult
                if err := json.Unmarshal(msg, &res); err == nil {
                    select {
                    case ch <- res:
                    case <-cctx.Done():
                        return
                    }
                    if res.Final {
                        // continue reading, provider may stream more
                    }
                    continue
                }
                // fallback: treat plain text as transcript
                select {
                case ch <- ASRResult{Text: string(msg), Final: false}:
                case <-cctx.Done():
                    return
                }
            }
        }
    }()

    return ch, nil
}

func (w *WSNetProvider) Stop(pcID string) {
    w.mu.Lock()
    if w.sessions != nil {
        if cancel, ok := w.sessions[pcID]; ok {
            cancel()
            delete(w.sessions, pcID)
        }
    }
    w.mu.Unlock()
}
