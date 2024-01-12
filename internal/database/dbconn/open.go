package dbconn

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log" //nolint:logging // TODO move all logging to sourcegraph/log
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/qustavo/sqlhooks/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var startupTimeout = func() time.Duration {
	str := env.Get("DB_STARTUP_TIMEOUT", "10s", "keep trying for this long to connect to PostgreSQL database before failing")
	d, err := time.ParseDuration(str)
	if err != nil {
		log.Fatalln("DB_STARTUP_TIMEOUT:", err)
	}
	return d
}()

var defaultMaxOpen = func() int {
	str := env.Get("SRC_PGSQL_MAX_OPEN", "30", "Maximum number of open connections to Postgres")
	v, err := strconv.Atoi(str)
	if err != nil {
		log.Fatalln("SRC_PGSQL_MAX_OPEN:", err)
	}
	return v
}()

var defaultMaxIdle = func() int {
	// For now, use the old default of max_idle == max_open
	str := env.Get("SRC_PGSQL_MAX_IDLE", "30", "Maximum number of idle connections to Postgres")
	v, err := strconv.Atoi(str)
	if err != nil {
		log.Fatalln("SRC_PGSQL_MAX_IDLE:", err)
	}
	return v
}()

func newWithConfig(cfg *pgx.ConnConfig) (*sql.DB, error) {
	db, err := openDBWithStartupWait(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "DB not available")
	}

	if err := ensureMinimumPostgresVersion(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func openDBWithStartupWait(cfg *pgx.ConnConfig) (db *sql.DB, err error) {
	// Allow the DB to take up to 10s while it reports "pq: the database system is starting up".
	startupDeadline := time.Now().Add(startupTimeout)
	for {
		if time.Now().After(startupDeadline) {
			return nil, errors.Wrapf(err, "database did not start up within %s", startupTimeout)
		}
		db, err = open(cfg)
		if err == nil {
			err = db.Ping()
		}
		if err != nil && isDatabaseLikelyStartingUp(err) {
			time.Sleep(startupTimeout / 10)
			continue
		}
		return db, err
	}
}

// extendedDriver wraps sqlHooks' driver to provide a conn that implements Ping, ResetSession
// and CheckNamedValue, which is mandatory as otelsql is instrumenting these methods.
// For all mandatory methods the sqlHooks driver is used. For the optional methods namely Ping, ResetSession and CheckNamedValue
// (which the sqlHooks driver does not implement), extendedConn goes to the original default driver.
//
//	                            Ping()
//	                            ResetSession()
//	                            CheckNamedValue()
//	                   ┌──────────────────────────────┐
//	                   │                              │
//	                   │                              │
//	                   │                              │
//	┌───────┐   ┌──────┴─────┐   ┌────────┐     ┌─────▼───────┐
//	│       │   │            │   │        │     │             │
//	│otelsql├──►│extendedConn├──►│sqlhooks├────►│DefaultDriver│
//	│       │   │            │   │        │     │             │
//	└─┬─────┘   └─┬──────────┘   └─┬──────┘     └─┬───────────┘
//	  │           │                │              │
//	  │           │                │              │Implements all SQL driver methods
//	  │           │                │
//	  │           │                │Only implements mandatory ones
//	  │           │                │Ping(), ResetSession() and CheckNamedValue() are missing.
//	  │           │
//	  │           │Implement all SQL driver methods
//	  │
//	  │Expects all SQL driver methods
//
// A sqlhooks.Driver must be used as a Driver otherwise errors will be raised.
type extendedDriver struct {
	driver.Driver
}

// extendedConn wraps sqlHooks' conn that does implement Ping, ResetSession and
// CheckNamedValue into one that does, by accessing the underlying conn from the
// original driver that does implement these methods.
type extendedConn struct {
	driver.Conn
	driver.ConnPrepareContext
	driver.ConnBeginTx

	execerContext  driver.ExecerContext
	queryerContext driver.QueryerContext
}

var _ driver.Pinger = &extendedConn{}
var _ driver.SessionResetter = &extendedConn{}
var _ driver.NamedValueChecker = &extendedConn{}

// Open returns a conn wrapped through extendedConn, implementing the
// Ping, ResetSession and CheckNamedValue optional methods that the
// otelsql.Conn expects to be implemented.
func (d *extendedDriver) Open(str string) (driver.Conn, error) {
	if _, ok := d.Driver.(*sqlhooks.Driver); !ok {
		return nil, errors.New("sql driver is not a sqlhooks.Driver")
	}

	if pgConnectionUpdater != "" {
		// Driver.Open() is called during after we first attempt to connect to the database
		// during startup time in `dbconn.open()`, where the manager will persist the config internally,
		// and also call the underlying pgx RegisterConnConfig() to register the config to pgx driver.
		// Therefore, this should never be nil.
		//
		// We do not need this code path unless connection updater is enabled.
		cfg := manager.getConfig(str)
		if cfg == nil {
			return nil, errors.Newf("no config found %q", str)
		}

		u, ok := connectionUpdaters[pgConnectionUpdater]
		if !ok {
			return nil, errors.Errorf("unknown connection updater %q", pgConnectionUpdater)
		}
		if u.ShouldUpdate(cfg) {
			config, err := u.Update(cfg.Copy())
			if err != nil {
				return nil, errors.Wrapf(err, "update connection %q", str)
			}
			str = manager.registerConfig(config)
		}
	}

	c, err := d.Driver.Open(str)
	if err != nil {
		return nil, err
	}

	// Ensure we're not casting things blindly.
	if _, ok := c.(any).(driver.ExecerContext); !ok {
		return nil, errors.New("sql conn doen't implement driver.ExecerContext")
	}
	if _, ok := c.(any).(driver.QueryerContext); !ok {
		return nil, errors.New("sql conn doen't implement driver.QueryerContext")
	}
	if _, ok := c.(any).(driver.Conn); !ok {
		return nil, errors.New("sql conn doen't implement driver.Conn")
	}
	if _, ok := c.(any).(driver.ConnPrepareContext); !ok {
		return nil, errors.New("sql conn doen't implement driver.ConnPrepareContext")
	}
	if _, ok := c.(any).(driver.ConnBeginTx); !ok {
		return nil, errors.New("sql conn doen't implement driver.ConnBeginTx")
	}

	// Build the extended connection.
	return &extendedConn{
		Conn:               c.(any).(driver.Conn),
		ConnPrepareContext: c.(any).(driver.ConnPrepareContext),
		ConnBeginTx:        c.(any).(driver.ConnBeginTx),
		execerContext:      c.(any).(driver.ExecerContext),
		queryerContext:     c.(any).(driver.QueryerContext),
	}, nil
}

// Access the underlying connection, so we can forward the methods that
// sqlhooks does not implement on its own.
func (n *extendedConn) rawConn() driver.Conn {
	c := n.Conn.(*sqlhooks.ExecerQueryerContextWithSessionResetter)
	return c.Conn.Conn
}

func (n *extendedConn) Ping(ctx context.Context) error {
	return n.rawConn().(driver.Pinger).Ping(ctx)
}

func (n *extendedConn) ResetSession(ctx context.Context) error {
	return n.rawConn().(driver.SessionResetter).ResetSession(ctx)
}

func (n *extendedConn) CheckNamedValue(namedValue *driver.NamedValue) error {
	return n.rawConn().(driver.NamedValueChecker).CheckNamedValue(namedValue)
}

func (n *extendedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	ctx, query = instrumentQuery(ctx, query, len(args))
	return n.execerContext.ExecContext(ctx, query, args)
}

func (n *extendedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	ctx, query = instrumentQuery(ctx, query, len(args))
	return n.queryerContext.QueryContext(ctx, query, args)
}

func registerPostgresProxy() {
	m := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_pgsql_request_total",
		Help: "Total number of SQL requests to the database.",
	}, []string{"type"})

	dri := sqlhooks.Wrap(stdlib.GetDefaultDriver(), combineHooks(
		&metricHooks{
			metricSQLSuccessTotal: m.WithLabelValues("success"),
			metricSQLErrorTotal:   m.WithLabelValues("error"),
		},
	))
	sql.Register("postgres-proxy", &extendedDriver{dri})
}

var registerOnce sync.Once

func open(cfg *pgx.ConnConfig) (*sql.DB, error) {
	registerOnce.Do(registerPostgresProxy)
	// this function is called once during startup time, and we register the db config
	// to our own manager, and manager will also register the config to pgx driver by
	// calling the underlying stdlib.RegisterConnConfig().
	name := manager.registerConfig(cfg)

	db, err := otelsql.Open(
		"postgres-proxy",
		name,
		otelsql.WithTracerProvider(otel.GetTracerProvider()),
		otelsql.WithSQLCommenter(true),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			OmitConnResetSession: true,
			OmitConnPrepare:      true,
			OmitRows:             true,
			OmitConnectorConnect: true,
		}),
		otelsql.WithAttributesGetter(argsAsAttributes),
	)
	if err != nil {
		return nil, errors.Wrap(err, "postgresql open")
	}

	// Set max open and idle connections
	maxOpen, _ := strconv.Atoi(cfg.RuntimeParams["max_conns"])
	if maxOpen == 0 {
		maxOpen = defaultMaxOpen
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(defaultMaxIdle)
	db.SetConnMaxIdleTime(time.Minute)

	return db, nil
}

// isDatabaseLikelyStartingUp returns whether the err likely just means the PostgreSQL database is
// starting up, and it should not be treated as a fatal error during program initialization.
func isDatabaseLikelyStartingUp(err error) bool {
	substrings := []string{
		// Wait for DB to start up.
		"the database system is starting up",
		// Wait for DB to start listening.
		"connection refused",
		"failed to receive message",
	}

	msg := err.Error()
	for _, substring := range substrings {
		if strings.Contains(msg, substring) {
			return true
		}
	}

	return false
}

const argsAttributesValueLimit = 100

// argsAsAttributes generates a set of OpenTelemetry trace attributes that represent the
// argument values used in a query.
func argsAsAttributes(ctx context.Context, _ otelsql.Method, _ string, args []driver.NamedValue) []attribute.KeyValue {
	// Do not decorate span with args as attributes if that's a bulk insertion
	// or if we have too many args (it's unreadable anyway).
	if isBulkInsertion(ctx) || len(args) > 24 {
		return []attribute.KeyValue{attribute.Bool("db.args.skipped", true)}
	}

	attrs := make([]attribute.KeyValue, len(args))
	for i, arg := range args {
		key := "db.args.$" + strconv.Itoa(arg.Ordinal)

		// Value is a value that drivers must be able to handle.
		// It is either nil, a type handled by a database driver's NamedValueChecker
		// interface, or an instance of one of these types:
		//
		//	int64
		//	float64
		//	bool
		//	[]byte
		//	string
		//	time.Time
		switch v := arg.Value.(type) {
		case nil:
			attrs[i] = attribute.String(key, "nil")
		case int64:
			attrs[i] = attribute.Int64(key, v)
		case float64:
			attrs[i] = attribute.Float64(key, v)
		case bool:
			attrs[i] = attribute.Bool(key, v)
		case []byte:
			attrs[i] = attribute.String(key, truncateStringValue(string(v)))
		case string:
			attrs[i] = attribute.String(key, truncateStringValue(v))
		case time.Time:
			attrs[i] = attribute.String(key, v.String())

		// pq.Array types
		case *pq.BoolArray:
			attrs[i] = attribute.BoolSlice(key, truncateSliceValue([]bool(*v)))
		case *pq.Float64Array:
			attrs[i] = attribute.Float64Slice(key, truncateSliceValue([]float64(*v)))
		case *pq.Float32Array:
			vals := truncateSliceValue([]float32(*v))
			floats := make([]float64, len(vals))
			for i, v := range vals {
				floats[i] = float64(v)
			}
			attrs[i] = attribute.Float64Slice(key, floats)
		case *pq.Int64Array:
			attrs[i] = attribute.Int64Slice(key, truncateSliceValue([]int64(*v)))
		case *pq.Int32Array:
			vals := truncateSliceValue([]int32(*v))
			ints := make([]int, len(vals))
			for i, v := range vals {
				ints[i] = int(v)
			}
			attrs[i] = attribute.IntSlice(key, ints)
		case *pq.StringArray:
			attrs[i] = attribute.StringSlice(key, truncateSliceValue([]string(*v)))
		case *pq.ByteaArray:
			vals := truncateSliceValue([][]byte(*v))
			strings := make([]string, len(vals))
			for i, v := range vals {
				strings[i] = string(v)
			}
			attrs[i] = attribute.StringSlice(key, strings)

		default: // in case we miss anything
			attrs[i] = attribute.String(key, fmt.Sprintf("%T", v))
		}
	}
	return attrs
}

func truncateStringValue(v string) string {
	if len(v) > argsAttributesValueLimit {
		return v[:argsAttributesValueLimit]
	}
	return v
}

func truncateSliceValue[T any](s []T) []T {
	if len(s) > argsAttributesValueLimit {
		return s[:argsAttributesValueLimit]
	}
	return s
}
