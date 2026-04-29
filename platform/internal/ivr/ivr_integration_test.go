package ivr

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/emiago/ai-call-center-platform/internal/call"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIVREnqueueAssignsSession(t *testing.T) {
	// setup in-memory DB
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&call.Session{}, &call.Tenant{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	svc := call.NewService(db, logger)

	// create session
	sess := call.Session{
		TenantID:  1,
		CallID:    "test-call-1",
		Direction: "outbound",
		State:     "queued",
	}
	if err := svc.CreateSession(context.Background(), &sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	engine := NewEngine(logger)
	// assign handler updates DB
	engine.SetAssignHandler(func(agentID string, s call.Session) {
		_ = svc.UpdateSessionState(context.Background(), s.ID, "assigned")
	})

	// register an agent and enqueue
	engine.RegisterAgent("agent-1")
	engine.Enqueue(sess)

	// wait for assignment to propagate
	time.Sleep(200 * time.Millisecond)

	got, err := svc.GetSessionByID(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.State != "assigned" {
		t.Fatalf("expected state assigned, got %s", got.State)
	}
}
