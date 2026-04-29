package ivr

import (
	"log/slog"
	"sync"

	"github.com/emiago/ai-call-center-platform/internal/call"
)

// Engine is a minimal in-memory IVR/ACD queue and agent registry.
type Engine struct {
	log *slog.Logger

	mu sync.Mutex
	// queue of sessions waiting to be assigned
	queue []call.Session

	// agents map agentID -> idle
	agents map[string]bool

	// assign callback invoked when an assignment is made
	onAssign func(agentID string, s call.Session)
}

func NewEngine(log *slog.Logger) *Engine {
	return &Engine{log: log, agents: map[string]bool{}, queue: []call.Session{}}
}

func (e *Engine) RegisterAgent(agentID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.agents[agentID] = true
}

func (e *Engine) ListAgents() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	res := make([]string, 0, len(e.agents))
	for id := range e.agents {
		res = append(res, id)
	}
	return res
}

// Enqueue adds session to the queue and attempts assignment
func (e *Engine) Enqueue(s call.Session) {
	e.mu.Lock()
	e.queue = append(e.queue, s)
	e.mu.Unlock()
	e.tryAssign()
}

func (e *Engine) SetAssignHandler(f func(agentID string, s call.Session)) {
	e.mu.Lock()
	e.onAssign = f
	e.mu.Unlock()
}

func (e *Engine) tryAssign() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.queue) == 0 || len(e.agents) == 0 {
		return
	}
	// pick first queued session and first agent
	s := e.queue[0]
	e.queue = e.queue[1:]
	var agentID string
	for id := range e.agents {
		agentID = id
		break
	}
	// invoke handler if present
	if e.onAssign != nil {
		go e.onAssign(agentID, s)
	}
}
