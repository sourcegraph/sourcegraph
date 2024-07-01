package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sourcegraph/run"

	_ "modernc.org/sqlite"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
)

const schemaVersion = "1"

type key int

const (
	invocationKey key = 0
	// eventKey      key = 1
)

type invocation struct {
	uuid uuid.UUID
}

var store = sync.OnceValue(func() analyticsStore {
	db, err := newDiskStore()
	if err != nil {
		std.Out.WriteWarningf("failed to create sg analytics store: %s", err)
	}
	return analyticsStore{db}
})

func newDiskStore() (*sql.DB, error) {
	sghome, err := root.GetSGHomePath()
	if err != nil {
		return nil, err
	}

	// this will create the file if it doesnt exist
	db, err := sql.Open("sqlite", "file://"+sghome+"analytics.sqlite")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS analytics (
		event_uuid TEXT PRIMARY KEY,
		-- invocation_uuid TEXT,
		schema_version TEXT NOT NULL,
		metadata_json TEXT,
	)`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

type analyticsStore struct {
	db *sql.DB
}

func (s analyticsStore) NewInvocation(ctx context.Context, uuid uuid.UUID, version string, meta map[string]any) {
	if s.db == nil {
		return
	}

	b, err := json.Marshal(meta)
	if err != nil {
		std.Out.WriteWarningf("invalid json generated for sg analytics metadata %v: %s", meta, err)
	}

	meta["email"] = email()
	meta["start_time"] = time.Now()

	_, err = s.db.Exec(`INSERT INTO analytics (event_uuid, schema_version, metadata_json) VALUES (?, ?, ?)`, uuid, schemaVersion, string(b))
	if err != nil {
		std.Out.WriteWarningf("failed to insert sg analytics event: %s", err)
	}
}

func (s analyticsStore) AddMetadata(ctx context.Context, uuid uuid.UUID, meta map[string]any) {
	if s.db == nil {
		return
	}

	b, err := json.Marshal(meta)
	if err != nil {
		std.Out.WriteWarningf("invalid json generated for sg analytics metadata %v: %s", meta, err)
		return
	}

	_, err = s.db.Exec(`UPDATE analytics SET metadata_json = json_patch(metadata_json, ?) WHERE event_uuid = ?`, string(b), uuid)
	if err != nil {
		std.Out.WriteWarningf("failed to update sg analytics event: %s", err)
	}
}

var email = sync.OnceValue[string](func() string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// Loose attempt at getting identity - if we fail, just discard
	identity, _ := run.Cmd(ctx, "git config user.email").StdOut().Run().String()
	return identity
})

func NewInvocation(ctx context.Context, version string, meta map[string]any) context.Context {
	u, _ := uuid.NewV7()
	invc := invocation{u}

	store().NewInvocation(ctx, u, version, meta)

	return context.WithValue(ctx, invocationKey, invc)
}

func AddMeta(ctx context.Context, meta map[string]any) {
	invc, ok := ctx.Value(invocationKey).(invocation)
	if !ok {
		return
	}

	store().AddMetadata(ctx, invc.uuid, meta)
}

func InvocationSucceeded(ctx context.Context) {
	invc, ok := ctx.Value(invocationKey).(invocation)
	if !ok {
		return
	}

	store().AddMetadata(ctx, invc.uuid, map[string]any{
		"success":  true,
		"end_time": time.Now(),
	})
}

func InvocationCancelled(ctx context.Context) {
	invc, ok := ctx.Value(invocationKey).(invocation)
	if !ok {
		return
	}

	store().AddMetadata(ctx, invc.uuid, map[string]any{
		"cancelled": true,
		"end_time":  time.Now(),
	})
}

func InvocationFailed(ctx context.Context, err error) {
	invc, ok := ctx.Value(invocationKey).(invocation)
	if !ok {
		return
	}

	store().AddMetadata(ctx, invc.uuid, map[string]any{
		"failed":   true,
		"error":    err.Error(),
		"end_time": time.Now(),
	})
}
