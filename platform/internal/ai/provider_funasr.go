package ai

import (
    "bufio"
    "context"
    "encoding/json"
    "io"
    "log/slog"
    "os/exec"
    "sync"
    "time"
)

// FunASRProvider runs a local funasr ONNX binary that accepts raw PCM on stdin
// and emits JSON or text lines on stdout containing partial/final transcripts.
type FunASRProvider struct {
    Cmd   string   // command to run (e.g., "funasr_local")
    Args  []string // optional args
    Transcoder Transcoder
    Logger *slog.Logger

    mu       sync.Mutex
    sessions map[string]context.CancelFunc
}

func NewFunASRProvider(cmd string, args []string, logger *slog.Logger) *FunASRProvider {
    return &FunASRProvider{Cmd: cmd, Args: args, Logger: logger}
}

func (f *FunASRProvider) Start(ctx context.Context, pcID string, r io.Reader) (<-chan ASRResult, error) {
    if f.Transcoder == nil {
        f.Transcoder = &IdentityTranscoder{}
    }
    rdr, err := f.Transcoder.Transcode(r)
    if err != nil {
        return nil, err
    }

    ch := make(chan ASRResult, 16)
    cctx, cancel := context.WithCancel(ctx)
    f.mu.Lock()
    if f.sessions == nil {
        f.sessions = map[string]context.CancelFunc{}
    }
    f.sessions[pcID] = cancel
    f.mu.Unlock()

    go func() {
        defer func() {
            f.mu.Lock()
            delete(f.sessions, pcID)
            f.mu.Unlock()
            close(ch)
        }()

        // start process
        cmd := exec.CommandContext(cctx, f.Cmd, f.Args...)
        stdin, err := cmd.StdinPipe()
        if err != nil {
            if f.Logger != nil { f.Logger.Error("funasr: stdin pipe", "err", err) }
            return
        }
        stdout, err := cmd.StdoutPipe()
        if err != nil {
            _ = stdin.Close()
            if f.Logger != nil { f.Logger.Error("funasr: stdout pipe", "err", err) }
            return
        }

        if err := cmd.Start(); err != nil {
            _ = stdin.Close()
            if f.Logger != nil { f.Logger.Error("funasr: start", "err", err) }
            return
        }

        // copy audio into process stdin
        go func() {
            defer stdin.Close()
            io.Copy(stdin, rdr)
        }()

        // read lines from stdout
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            line := scanner.Bytes()
            // try parse JSON
            var res ASRResult
            if err := json.Unmarshal(line, &res); err == nil {
                select {
                case ch <- res:
                case <-cctx.Done():
                    return
                }
                if res.Final {
                    // continue
                }
                continue
            }
            // treat plain text as transcript
            select {
            case ch <- ASRResult{Text: string(line), Final: false}:
            case <-cctx.Done():
                return
            }
        }

        // wait for process or cancellation
        done := make(chan struct{})
        go func() { cmd.Wait(); close(done) }()

        select {
        case <-cctx.Done():
            // try graceful kill then force
            _ = cmd.Process.Kill()
            return
        case <-done:
            // process exited, send final empty if needed
            select {
            case ch <- ASRResult{Text: "", Final: true}:
            case <-time.After(200 * time.Millisecond):
            }
            return
        }
    }()

    return ch, nil
}

func (f *FunASRProvider) Stop(pcID string) {
    f.mu.Lock()
    if f.sessions != nil {
        if cancel, ok := f.sessions[pcID]; ok {
            cancel()
            delete(f.sessions, pcID)
        }
    }
    f.mu.Unlock()
}
