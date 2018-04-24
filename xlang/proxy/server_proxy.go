package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	log15 "gopkg.in/inconshreveable/log15.v2"

	basictracer "github.com/opentracing/basictracer-go"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sourcegraph/go-langserver/pkg/lsp"
	plspext "github.com/sourcegraph/go-langserver/pkg/lspext"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/sourcegraph/sourcegraph/pkg/api"
	"github.com/sourcegraph/sourcegraph/pkg/env"
	"github.com/sourcegraph/sourcegraph/xlang/lspext"
)

var repoLargeSubstr = strings.Fields(env.Get("REPO_LARGE_SUBSTR", "", "repo substrings which should be sent to LS with the mode suffix _large. Separated by whitespace"))

// serverID identifies a lang/build server by the minimal state
// necessary to reinitialize it. At most one lang/build server per
// serverID will be used; if two clients issue requests that route to
// the same serverID, their requests will be sent to the same
// lang/build server.
type serverID struct {
	contextID
}

func (id serverID) String() string {
	return fmt.Sprintf("server(%s)", id.contextID)
}

type serverProxyConn struct {
	conn *jsonrpc2.Conn // the LSP JSON-RPC 2.0 connection to the server

	doneOnce sync.Once     // protects closing done
	done     chan struct{} // when closed the connection will be shutdown

	id serverID

	// clientBroadcast is used to forward incoming requests from servers
	// to clients.
	clientBroadcast func(context.Context, *jsonrpc2.Request)

	// initOnce ensures we only connect and initialize once, and other
	// goroutines wait until the 1st goroutine completes those tasks.
	initOnce sync.Once
	// initResult and initErr are only safe to write inside initOnce.Do(...), only safe to read after calling initOnce.Do(...)
	initResult *lsp.InitializeResult
	initErr    error

	mu          sync.Mutex
	rootFS      FileSystem // the workspace's file system
	stats       serverProxyConnStats
	diagnostics map[diagnosticsKey][]lsp.Diagnostic // saved diagnostics
	messages    []lsp.ShowMessageParams             // saved messages
}

// serverProxyConnStats contains statistics for a proxied connection to a server.
type serverProxyConnStats struct {
	// Created is the time the proxy connection was created
	Created time.Time

	// Last is max(last request sent, last response received), used to
	// disconnect from unused servers
	Last time.Time

	// TotalCount is the total number of calls proxied to the server.
	TotalCount int

	// Counts is the total number of calls proxied to the server per
	// LSP method.
	Counts map[string]int

	// TotalFinishedCount is the total number of calls proxied to the
	// server which finished. This is closely related to TotalCount,
	// except is only incremented once a result is returned.
	TotalFinishedCount int

	// TotalErrorCount is the total number of calls proxied to the server
	// that failed.
	TotalErrorCount int

	// ErrorCounts is the total number of calls proxied to the server that
	// failed per LSP method.
	ErrorCounts map[string]int

	// NOTE: If you add a new field here, please ensure `Stats()` works
	// correctly under concurrent read/writes.
}

var (
	serverConnsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "src",
		Subsystem: "xlang",
		Name:      "open_lsp_server_connections",
		Help:      "Open connections (initialized + uninitialized) to the LSP servers.",
	}, []string{"mode"})
	serverConnsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "src",
		Subsystem: "xlang",
		Name:      "cumu_lsp_server_connections",
		Help:      "Cumulative number of connections (initialized + uninitialized) to the LSP servers (total of open + previously closed since process startup).",
	}, []string{"mode"})
	serverConnsMethodCalls = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "src",
		Subsystem: "xlang",
		Name:      "cumu_lsp_server_method_calls",
		Help:      "Total number of calls sent for a (method, mode).",
	}, []string{"mode", "method"})
	serverConnsTotalMethodCalls = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "src",
		Subsystem: "xlang",
		Name:      "lsp_server_method_calls",
		Help:      "Total number of calls sent to a server proxy before it is shutdown.",
		Buckets:   []float64{1, 2, 4, 8, 16, 32, 64, 128, 256},
	}, []string{"mode"})
	serverConnsFailedMethodCalls = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "src",
		Subsystem: "xlang",
		Name:      "lsp_server_failed_method_calls",
		Help:      "Total number of failed calls sent to a server proxy before it is shutdown.",
		Buckets:   []float64{0.1, 1, 2, 4, 8, 16, 32, 64, 128, 256},
	}, []string{"mode"})
	serverConnsAliveDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "src",
		Subsystem: "xlang",
		Name:      "lsp_server_alive_seconds",
		Help:      "The number of seconds a proxied connection is kept alive.",
		Buckets:   []float64{1, 10, 300, 2 * 300, 3 * 300, 4 * 300}, // 300 is the default MaxServerIdle
	}, []string{"mode"})
)

func init() {
	prometheus.MustRegister(serverConnsGauge)
	prometheus.MustRegister(serverConnsCounter)
	prometheus.MustRegister(serverConnsMethodCalls)
	prometheus.MustRegister(serverConnsTotalMethodCalls)
	prometheus.MustRegister(serverConnsFailedMethodCalls)
	prometheus.MustRegister(serverConnsAliveDuration)
}

// ShutdownServers shuts down all open servers. This is exported to be used by
// tests so they do not need to wait for MaxServerIdle to be reached.
func (p *Proxy) ShutdownServers(ctx context.Context) error {
	return p.shutdownServers(ctx, func(*serverProxyConn) bool { return true })
}

// shutdownServers shuts down servers whose filter function returns
// true. Note: Proxy.mu is held when filter is called.
func (p *Proxy) shutdownServers(ctx context.Context, filter func(*serverProxyConn) bool) error {
	var shutdown []*serverProxyConn
	p.mu.Lock()
	for s := range p.servers {
		if filter(s) {
			shutdown = append(shutdown, s)
			delete(p.servers, s)
		}
	}
	p.mu.Unlock()

	if len(shutdown) == 0 {
		return nil
	}

	errs := &errorList{}
	for _, s := range shutdown {
		err := s.Close()
		if err != nil {
			errs.add(err)
		}
	}
	return errs.error()
}

// shutDownServer will terminate the server matching ID. If no such server
// exists, no action is taken.
func (p *Proxy) shutDownServer(ctx context.Context, id serverID) error {
	return p.shutdownServers(ctx, func(c *serverProxyConn) bool {
		return c.id == id
	})
}

// LogServerStats, if true, will log the statistics of a serverProxyConn when
// it is removed.
var LogServerStats = true

func (p *Proxy) removeServerConn(c *serverProxyConn) {
	p.mu.Lock()
	_, ok := p.servers[c]
	if ok {
		delete(p.servers, c)
	}
	p.mu.Unlock()
	if ok {
		c.didRemove()
	}
}

// getServerConn returns an existing connection to the specified
// server or creates one if none exists.
func (p *Proxy) getServerConn(ctx context.Context, id serverID) (c *serverProxyConn, initResult *lsp.InitializeResult, err error) {
	// Check for an already established connection.
	p.mu.Lock()
	for cc := range p.servers {
		if cc.id == id {
			p.mu.Unlock()
			c = cc
			break
		}
	}

	// No connection found, so we need to create one.
	if c == nil {
		// We're still holding p.mu. Do the minimum work necessary
		// here to be able to safely unlock it, so we don't block the entire
		// proxy.
		c = &serverProxyConn{
			id:              id,
			done:            make(chan struct{}),
			clientBroadcast: p.clientBroadcastFunc(id.contextID),
			stats: serverProxyConnStats{
				Created: time.Now(),
				Last:    time.Now(),
			},
		}
		p.servers[c] = struct{}{}
		serverConnsGauge.WithLabelValues(id.mode).Inc()
		serverConnsCounter.WithLabelValues(id.mode).Inc()
		p.mu.Unlock()
	}

	// No longer holding p.mu.

	// Connect and initialize.
	didWeInit := false // whether WE (not another goroutine) actually executed the c.initOnce.Do(...) func
	c.initOnce.Do(func() {
		didWeInit = true

		// Best effort cleanup of resources when we fail to connect.
		var rwc io.ReadWriteCloser
		defer func() {
			if c.initErr == nil {
				return
			}
			if fs, ok := c.rootFS.(io.Closer); ok && fs != nil {
				_ = fs.Close()
			}
			if c.conn != nil {
				_ = c.conn.Close()
			}
			if rwc != nil {
				_ = rwc.Close()
			}
		}()

		// SECURITY NOTE: We assume that the caller to the LSP client
		// proxy has already checked the user's permissions to read
		// this repo, so we don't need to check permissions again
		// here.
		//
		// Do this check early in case the rev does not exist / the repo is no
		// longer cloneable.
		var err error
		c.rootFS, err = NewRemoteRepoVFS(ctx, id.rootURI.CloneURL(), api.CommitID(id.rootURI.Rev()))
		if err != nil {
			c.initErr = err
			return
		}

		mode := id.mode
		repo := id.rootURI.Repo()

		if p.shouldUseLargeServer(mode, repo) {
			mode = mode + "_large"
		}

		if p.shouldStripBGMode(mode) {
			mode = strings.TrimSuffix(mode, "_bg")
		}

		rwc, err = connectToServer(ctx, mode)
		if err != nil {
			c.initErr = err
			return
		}
		c.updateLastTime()

		var connOpt []jsonrpc2.ConnOpt
		if p.Trace {
			connOpt = append(connOpt, jsonrpc2.LogMessages(log.New(os.Stderr, "", 0)))
		}
		c.conn = jsonrpc2.NewConn(ctx, jsonrpc2.NewBufferedStream(rwc, jsonrpc2.VSCodeObjectCodec{}), jsonrpc2.AsyncHandler(jsonrpc2.HandlerWithError(c.handle)), connOpt...)

		c.initResult, c.initErr = c.lspInitialize(ctx)
		if c.initErr != nil {
			return
		}
		c.updateLastTime()

		// When the connection goes away remove from the connection list and
		// free up associated resources (e.g., if the VFS is backed by a file
		// on disk, this will close the file).
		go func() {
			var doShutdown bool
			select {
			case <-c.conn.DisconnectNotify():
				// Server conn gone so can't do LSP shutdown.
				doShutdown = false
			case <-c.done:
				// Told to shutdown, do best effort LSP shutdown.
				doShutdown = true
			}

			// Remove ourselves from the list ASAP to prevent other clients
			// using the connection. We do this in its own goroutine so we can
			// proceed with shutdown even if someone else holds the proxy
			// mutex.
			go p.removeServerConn(c)

			if doShutdown {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				if err := c.conn.Call(ctx, "shutdown", nil, nil); err != nil {
					// The only interesting error to log is application level
					// errors and timeouts. Connection errors are not
					// interesting due to timeout.
					if _, isAppError := err.(*jsonrpc2.Error); isAppError || err == context.DeadlineExceeded {
						log15.Debug("shutdown request failed", "id", c.id, "error", err)
					}
				}
				// no application level errors for notifications, so ignore error
				_ = c.conn.Notify(ctx, "exit", nil)
			}

			c.conn.Close()
			if fs, ok := c.rootFS.(io.Closer); ok && fs != nil {
				_ = fs.Close()
			}
		}()
	})

	err = c.initErr
	if err != nil {
		if didWeInit {
			// If we encounter an error during initialization, fail every
			// other goroutine that was waiting at the time (with the same
			// error), but don't prevent future goroutines from retrying (in
			// case of ephemeral errors).
			p.removeServerConn(c)
		} else {
			// Make it clear that we're just passing along an error
			// that another goroutine received, so it doesn't seem
			// (from the error logs) that we performed the same
			// network/etc. operation many times.
			//
			// Preserve the error code so the caller knows the kind of error this is.
			const otherGoroutineMessage = "other goroutine failed to connect and initialize LSP server"
			if e, ok := err.(*jsonrpc2.Error); ok {
				err = &jsonrpc2.Error{
					Message: otherGoroutineMessage + ": " + e.Message,
					Code:    e.Code,
				}
			} else {
				err = errors.Wrap(err, otherGoroutineMessage)
			}
		}
		return nil, nil, err
	}

	return c, c.initResult, nil
}

func (p *Proxy) shouldUseLargeServer(mode string, repo api.RepoURI) bool {
	if _, hasLargeMode := ServersByMode[mode+"_large"]; hasLargeMode {
		for _, p := range repoLargeSubstr {
			if strings.Contains(string(repo), p) {
				return true
			}
		}
	}
	return false
}

func (p *Proxy) shouldStripBGMode(mode string) bool {
	if !strings.HasSuffix(mode, "_bg") {
		return false
	}

	_, hasBGMode := ServersByMode[mode]
	return !hasBGMode
}

// initializeServer will ensure we either have an open connection or will open
// one to ID. It returns the initializeResult as a json.RawMessage. If it
// fails, it will return a non-nil error.
func (p *Proxy) initializeServer(ctx context.Context, id serverID) (*lsp.InitializeResult, error) {
	_, initResult, err := p.getServerConn(ctx, id)
	return initResult, err
}

// clientBroadcastFunc returns a function which will broadcast a request to
// all active clients for id.
func (p *Proxy) clientBroadcastFunc(id contextID) func(context.Context, *jsonrpc2.Request) {
	return func(ctx context.Context, req *jsonrpc2.Request) {
		// TODO(sqs): some clients will have already received these
		p.mu.Lock()
		for cc := range p.clients {
			if cc.context == id {
				// Ignore errors for forwarding diagnostics.
				go cc.handleFromServer(ctx, cc.conn, req)
			}
		}
		p.mu.Unlock()
	}
}

func (c *serverProxyConn) lspInitialize(ctx context.Context) (*lsp.InitializeResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "LSP server proxy: initialize",
		opentracing.Tags{"mode": c.id.mode, "rootURI": c.id.rootURI.String()},
	)
	defer span.Finish()
	timeout := 30 * time.Second
	if c.id.mode == "php" || c.id.mode == "php_bg" {
		timeout = 3 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	initParams := lspext.InitializeParams{
		InitializeParams: lsp.InitializeParams{
			RootPath: "/",
			RootURI:  "file:///",
			Capabilities: lsp.ClientCapabilities{
				XFilesProvider:   true,
				XContentProvider: true,
				XCacheProvider:   true,
			},
		},
		OriginalRootURI: lsp.DocumentURI(c.id.rootURI.String()),
		Mode:            c.id.mode,
	}
	if initOps := getInitializationOptions(c.id.mode); initOps != nil {
		initParams.InitializationOptions = initOps
	}
	var res lsp.InitializeResult
	err := c.conn.Call(ctx, "initialize", initParams, &res, addTraceMeta(ctx))
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// callServer sends an LSP request to the specified server
// (establishing the connection first if necessary).
func (p *Proxy) callServer(ctx context.Context, crid clientRequestID, sid serverID, method string, notif, requestOriginatedFromProxy bool, params, result interface{}) (err error) {
	var c *serverProxyConn

	span, ctx := opentracing.StartSpanFromContext(ctx, "LSP server proxy: "+method,
		opentracing.Tags{"mode": sid.mode, "rootURI": sid.rootURI.String(), "method": method, "params": params},
	)
	defer func() {
		if c != nil {
			c.incTotalFinishedStat()
		}
		if err != nil {
			ext.Error.Set(span, true)
			span.LogFields(otlog.Error(err))
			if c != nil {
				c.incMethodErrorStat(method)
			}
		}
		span.Finish()
	}()

	c, _, err = p.getServerConn(ctx, sid)
	if err != nil {
		return err
	}
	c.updateLastTime()
	c.incMethodStat(method)

	callOpts := []jsonrpc2.CallOption{addTraceMeta(ctx)}
	if notif {
		span.LogFields(otlog.String("event", "sending notification"))
		return c.conn.Notify(ctx, method, params, callOpts...)
	}
	if !requestOriginatedFromProxy {
		// See (*clientProxyConn).callServer for the meaning of
		// requestOriginatedFromProxy. We only want to tie back this
		// request to the client if it actually originated from a
		// client.
		callOpts = append(callOpts, jsonrpc2.PickID(crid.ID()))
	}
	span.LogFields(otlog.String("event", "sending request"))
	return c.conn.Call(ctx, method, params, result, callOpts...)
}

// traceFSRequests is whether to trace the LSP proxy's incoming
// requests for fs/readFile, fs/readDir, fs/stat, fs/lstat, etc. It is
// off by default because there are a lot of these and the traces can
// get quite noisy if it's enabled, but it is configurable because
// it's useful when you are debugging certain perf issues.
var traceFSRequests, _ = strconv.ParseBool(os.Getenv("LSP_PROXY_TRACE_FS_REQUESTS"))

func (c *serverProxyConn) handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (result interface{}, err error) {
	c.updateLastTime()
	defer c.updateLastTime()

	// Trace the handling of this request. Only create child spans for
	// significant operations, not when we're just receiving traces or
	// performing simple VFS ops.
	var span opentracing.Span
	if shouldCreateChildSpan := !isInfraMethod(req.Method) && (traceFSRequests || !isFSMethod(req.Method)); shouldCreateChildSpan {
		op := "LSP server proxy: handle " + req.Method

		// Try to get our parent span context from this JSON-RPC request's metadata.
		if req.Meta != nil {
			var carrier opentracing.TextMapCarrier
			if err := json.Unmarshal(*req.Meta, &carrier); err != nil {
				return nil, err
			}
			if clientCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, carrier); err == nil {
				span = opentracing.StartSpan(op, ext.RPCServerOption(clientCtx))
				span.SetTag("method", req.Method)
				defer func() {
					if err != nil {
						ext.Error.Set(span, true)
						span.LogFields(otlog.Error(err))
					}
					span.Finish()
				}()
				ctx = opentracing.ContextWithSpan(ctx, span)
			} else if err != opentracing.ErrSpanContextNotFound {
				return nil, err
			}
		}
	}

	switch req.Method {
	case "telemetry/event":
		if req.Params == nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
		}
		o, ok := opentracing.GlobalTracer().(basictracer.Tracer)
		if !ok {
			return nil, nil
		}
		r := o.Options().Recorder
		if r == nil {
			return nil, nil
		}
		var rawSpan basictracer.RawSpan
		if err := json.Unmarshal(*req.Params, &rawSpan); err != nil {
			return nil, err
		}
		r.RecordSpan(rawSpan)
		return nil, nil

	case "fs/readFile", "fs/readDirFiles", "fs/readDir", "fs/stat", "fs/lstat":
		if req.Params == nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
		}
		var path string
		if err := json.Unmarshal(*req.Params, &path); err != nil {
			return nil, err
		}
		if span != nil {
			span.SetTag("path", path)
		}
		return c.handleFS(ctx, req.Method, path)

	case "window/showMessage":
		// We pass through messages from language servers to ALL clients. We assume that 99.9% of `window/showMessage`
		// requests from the server will be applicable to all clients. Currently, this is true, because not that many
		// language servers issue `window/showMessage` requests, but this assumption may need to change in the future.
		//
		// Messages with the `Save` field true are saved to be displayed to any new client that connects using the same
		// server connection.

		var params lsp.ShowMessageParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}

		// When we start using stateful connections to the client again, we
		// should consider the save + broadcast below.
		// c.saveMessage(params)
		// c.clientBroadcast(ctx, req)

		logWithLevel(int(params.Type), "window/showMessage "+params.Message, c.id.contextID, "method", req.Method, "id", req.ID)
		return nil, nil

	case "window/logMessage":
		if req.Params == nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
		}
		var m lsp.LogMessageParams
		if err := json.Unmarshal(*req.Params, &m); err != nil {
			return nil, err
		}
		logWithLevel(int(m.Type), "window/logMessage "+m.Message, c.id.contextID, "method", req.Method, "id", req.ID)
		// Log to the span for the server, not for this specific
		// request.
		if span := opentracing.SpanFromContext(ctx); span != nil {
			span.LogFields(otlog.Object("window/logMessage", m))
		}
		return nil, nil

	case "textDocument/publishDiagnostics":
		if req.Params == nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
		}

		// Save for clients that connect later (who would not
		// otherwise receive the diagnostics, since the lang server
		// has no way to know to resend them).
		if proxySaveDiagnostics {
			var params lsp.PublishDiagnosticsParams
			if err := json.Unmarshal(*req.Params, &params); err != nil {
				return nil, err
			}
			c.saveDiagnostics(params)
		}

		// Forward to all clients.
		c.clientBroadcast(ctx, req)

		return nil, nil

	case "xcache/get":
		if req.Params == nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
		}
		var p plspext.CacheGetParams
		if err := json.Unmarshal(*req.Params, &p); err != nil {
			return nil, err
		}
		return c.handleCacheGet(ctx, p)

	case "xcache/set":
		// notification, so ignore errors
		if req.Params == nil {
			return nil, nil
		}
		var p plspext.CacheSetParams
		if err := json.Unmarshal(*req.Params, &p); err != nil {
			return nil, nil
		}
		c.handleCacheSet(ctx, p)
		return

	case "textDocument/xcontent":
		return c.handleTextDocumentContentExt(ctx, req)

	case "workspace/xfiles":
		return c.handleWorkspaceFilesExt(ctx, req)

	case "$/partialResult":
		// The partialResult is for a specific client, but we
		// broadcast this to all clients. It is expected
		// clientBroadcast implementations will correctly filter out
		// unrelated partialResults (by inspecting the ID)
		c.clientBroadcast(ctx, req)
		return
	}

	return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeMethodNotFound, Message: fmt.Sprintf("server proxy handler: method not found: %q", req.Method)}
}

func isFSMethod(method string) bool {
	return strings.HasPrefix(method, "fs/") || method == "textDocument/xcontent" || method == "workspace/xfiles"
}

// isInfraMethod returns true for methods which do not affect the end
// user. These are methods related to telemetry/logging/etc. Generally these
// are not useful to log.
func isInfraMethod(method string) bool {
	return method == "telemetry/event" || method == "window/logMessage" || method == "textDocument/publishDiagnostics"
}

// didRemove records statistics when shutting down server. It should only be
// called (once) when the server is being shutdown.
func (c *serverProxyConn) didRemove() {
	stats := c.Stats()
	serverConnsGauge.WithLabelValues(c.id.mode).Dec()
	serverConnsTotalMethodCalls.WithLabelValues(c.id.mode).Observe(float64(stats.TotalCount))
	serverConnsFailedMethodCalls.WithLabelValues(c.id.mode).Observe(float64(stats.TotalErrorCount))
	serverConnsAliveDuration.WithLabelValues(c.id.mode).Observe(stats.Last.Sub(stats.Created).Seconds())
	recordClosedServerConn(c.id, stats)
	if LogServerStats {
		logInfo("Removed serverProxyConn", c.id.contextID, "stats", stats)
	}
}

func (c *serverProxyConn) updateLastTime() {
	c.mu.Lock()
	c.stats.Last = time.Now()
	c.mu.Unlock()
}

func (c *serverProxyConn) incTotalFinishedStat() {
	c.mu.Lock()
	c.stats.TotalFinishedCount++
	c.mu.Unlock()
}

func (c *serverProxyConn) incMethodStat(method string) {
	serverConnsMethodCalls.WithLabelValues(c.id.mode, method).Inc()
	c.mu.Lock()
	c.stats.TotalCount++
	if c.stats.Counts == nil {
		c.stats.Counts = make(map[string]int)
	}
	c.stats.Counts[method] = c.stats.Counts[method] + 1
	c.mu.Unlock()
}

func (c *serverProxyConn) incMethodErrorStat(method string) {
	c.mu.Lock()
	c.stats.TotalErrorCount++
	if c.stats.ErrorCounts == nil {
		c.stats.ErrorCounts = make(map[string]int)
	}
	c.stats.ErrorCounts[method] = c.stats.ErrorCounts[method] + 1
	c.mu.Unlock()
}

func (c *serverProxyConn) Stats() serverProxyConnStats {
	c.mu.Lock()
	s := c.stats
	s.Counts = make(map[string]int)
	s.ErrorCounts = make(map[string]int)
	for k, v := range c.stats.Counts {
		s.Counts[k] = v
	}
	for k, v := range c.stats.ErrorCounts {
		s.ErrorCounts[k] = v
	}
	c.mu.Unlock()
	return s
}

// Close will close and free resources associated with the server. It will
// also send shutdown and notify to the server if it is running. This is done
// in its own goroutine.
func (c *serverProxyConn) Close() error {
	c.doneOnce.Do(func() {
		close(c.done)
	})
	return nil
}

var proxySaveDiagnostics, _ = strconv.ParseBool(env.Get("LSP_PROXY_SAVE_DIAGNOSTICS", "false", "save diagnostics published for each file to send to subsequently connected clients"))

type diagnosticsKey struct {
	serverID
	documentURI lsp.DocumentURI
}

// saveDiagnostics saves diagnostics to publish to clients that
// connect after they were sent. The language server does not have any
// way of resending diagnostics to newly connected clients, because
// the LSP proxy abstracts client connections away from the language
// server.
//
// Only the last diagnostics for a given file is valid.
func (c *serverProxyConn) saveDiagnostics(diagnostics lsp.PublishDiagnosticsParams) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.diagnostics == nil {
		c.diagnostics = map[diagnosticsKey][]lsp.Diagnostic{}
	}
	c.diagnostics[diagnosticsKey{serverID: c.id, documentURI: diagnostics.URI}] = diagnostics.Diagnostics
}

// saveMessage saves a message to publish to clients that connect after the message was sent. Certain messages (e.g.,
// build errors) should be shown to all clients, but the language server does not have a way of sending these to newly
// connected clients, because the LSP proxy abstracts client connections away from the language server. Unlike
// saveDiagnostics, saveMessage appends to the existing array of messages.
func (c *serverProxyConn) saveMessage(message lsp.ShowMessageParams) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, message)
}
