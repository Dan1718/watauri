package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	modernsqlite "modernc.org/sqlite"
)

// WATAURI_TRACE=1 enables verbose operation and SQL tracing. SQL arguments and
// message/contact contents are deliberately never recorded.
var traceEnabled = strings.TrimSpace(os.Getenv("WATAURI_TRACE")) == "1"
var traceLogWriteNanos atomic.Int64
var appLogWriteNanos atomic.Int64
var traceSequence atomic.Uint64

type traceIDKey struct{}
type traceSpanKey struct{}

type traceRWMutex struct {
	mu                     sync.RWMutex
	readCount, readTotal   atomic.Int64
	readMax                atomic.Int64
	writeCount, writeTotal atomic.Int64
	writeMax               atomic.Int64
}

func recordMaximum(target *atomic.Int64, value int64) {
	for old := target.Load(); value > old; old = target.Load() {
		if target.CompareAndSwap(old, value) {
			return
		}
	}
}

func (m *traceRWMutex) Lock() {
	if !traceEnabled {
		m.mu.Lock()
		return
	}
	started := time.Now()
	m.mu.Lock()
	if traceEnabled {
		wait := time.Since(started).Nanoseconds()
		m.writeCount.Add(1)
		m.writeTotal.Add(wait)
		recordMaximum(&m.writeMax, wait)
	}
}

func (m *traceRWMutex) Unlock() { m.mu.Unlock() }

func (m *traceRWMutex) RLock() {
	if !traceEnabled {
		m.mu.RLock()
		return
	}
	started := time.Now()
	m.mu.RLock()
	if traceEnabled {
		wait := time.Since(started).Nanoseconds()
		m.readCount.Add(1)
		m.readTotal.Add(wait)
		recordMaximum(&m.readMax, wait)
	}
}

func (m *traceRWMutex) RUnlock() { m.mu.RUnlock() }

func newTraceID() string {
	var id [8]byte
	if _, err := rand.Read(id[:]); err == nil {
		return hex.EncodeToString(id[:])
	}
	return hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
}

func traceID(ctx context.Context) string {
	if ctx != nil {
		if id, ok := ctx.Value(traceIDKey{}).(string); ok {
			return id
		}
	}
	return ""
}

func traceStart(ctx context.Context, name string, fields map[string]any) (context.Context, func(error)) {
	if !traceEnabled {
		return ctx, func(error) {}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id := traceID(ctx)
	if id == "" {
		id = newTraceID()
		ctx = context.WithValue(ctx, traceIDKey{}, id)
	}
	spanID := traceSequence.Add(1)
	if parent, ok := ctx.Value(traceSpanKey{}).(uint64); ok {
		fieldsWithParent := map[string]any{}
		for key, value := range fields {
			fieldsWithParent[key] = value
		}
		fieldsWithParent["parent_span_id"] = parent
		fields = fieldsWithParent
	}
	ctx = context.WithValue(ctx, traceSpanKey{}, spanID)
	started := time.Now()
	return ctx, func(err error) {
		attrs := map[string]any{"event": "span", "trace_id": id, "span_id": spanID, "name": name, "duration_ns": time.Since(started).Nanoseconds()}
		for key, value := range fields {
			attrs[key] = value
		}
		if err != nil {
			attrs["failed"] = true
			attrs["error_type"] = fmt.Sprintf("%T", err)
		}
		writeTrace(attrs)
	}
}

func traceEvent(ctx context.Context, name string, fields map[string]any) {
	if !traceEnabled {
		return
	}
	attrs := map[string]any{"event": name, "sequence": traceSequence.Add(1)}
	if id := traceID(ctx); id != "" {
		attrs["trace_id"] = id
	}
	if spanID, ok := ctx.Value(traceSpanKey{}).(uint64); ok {
		attrs["span_id"] = spanID
	}
	for key, value := range fields {
		attrs[key] = value
	}
	writeTrace(attrs)
}

func writeTrace(fields map[string]any) {
	if !traceEnabled {
		return
	}
	fields["previous_trace_emit_ns"] = traceLogWriteNanos.Swap(0)
	fields["previous_log_write_ns"] = appLogWriteNanos.Load()
	encoded, err := json.Marshal(fields)
	if err != nil {
		return
	}
	started := time.Now()
	_ = log.Output(2, "[trace] "+string(encoded))
	traceLogWriteNanos.Store(time.Since(started).Nanoseconds())
}

type timedLogWriter struct {
	mu  sync.Mutex
	out io.Writer
}

func (w *timedLogWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	started := time.Now()
	n, err := w.out.Write(data)
	appLogWriteNanos.Store(time.Since(started).Nanoseconds())
	return n, err
}

func enableLogWriteTiming() {
	if traceEnabled {
		log.SetOutput(&timedLogWriter{out: os.Stderr})
	}
}

func startStoreLockTrace() {
	if !traceEnabled {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			readCount := store.mu.readCount.Swap(0)
			readTotal := store.mu.readTotal.Swap(0)
			readMax := store.mu.readMax.Swap(0)
			writeCount := store.mu.writeCount.Swap(0)
			writeTotal := store.mu.writeTotal.Swap(0)
			writeMax := store.mu.writeMax.Swap(0)
			if readCount+writeCount == 0 {
				continue
			}
			traceEvent(context.Background(), "store_lock_interval", map[string]any{
				"interval_ms": 1000, "read_count": readCount, "read_wait_total_ns": readTotal, "read_wait_max_ns": readMax,
				"write_count": writeCount, "write_wait_total_ns": writeTotal, "write_wait_max_ns": writeMax,
			})
		}
	}()
}

// tracedSQLiteDriver wraps modernc SQLite at the database/sql driver boundary,
// so query, execution, iteration, and statement timings cover all store calls.
type tracedSQLiteDriver struct{ base driver.Driver }

func (d tracedSQLiteDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return tracedSQLiteConn{Conn: conn}, nil
}

type tracedSQLiteConn struct{ driver.Conn }

func (c tracedSQLiteConn) Prepare(query string) (driver.Stmt, error) {
	started := time.Now()
	stmt, err := c.Conn.Prepare(query)
	traceSQL(context.Background(), query, "prepare", time.Since(started), err)
	if err != nil {
		return nil, err
	}
	return tracedSQLiteStmt{Stmt: stmt, query: query}, nil
}

func (c tracedSQLiteConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if conn, ok := c.Conn.(driver.ConnPrepareContext); ok {
		started := time.Now()
		stmt, err := conn.PrepareContext(ctx, query)
		traceSQL(ctx, query, "prepare", time.Since(started), err)
		if err != nil {
			return nil, err
		}
		return tracedSQLiteStmt{Stmt: stmt, query: query}, nil
	}
	return c.Prepare(query)
}

func (c tracedSQLiteConn) Begin() (driver.Tx, error) {
	started := time.Now()
	tx, err := c.Conn.Begin()
	traceSQL(context.Background(), "BEGIN", "exec", time.Since(started), err)
	return tx, err
}

func (c tracedSQLiteConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if conn, ok := c.Conn.(driver.ConnBeginTx); ok {
		started := time.Now()
		tx, err := conn.BeginTx(ctx, opts)
		traceSQL(ctx, "BEGIN", "exec", time.Since(started), err)
		return tx, err
	}
	return c.Begin()
}

func (c tracedSQLiteConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if conn, ok := c.Conn.(driver.ExecerContext); ok {
		started := time.Now()
		result, err := conn.ExecContext(ctx, query, args)
		traceSQL(ctx, query, "exec", time.Since(started), err)
		return result, err
	}
	return nil, driver.ErrSkip
}

func (c tracedSQLiteConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if conn, ok := c.Conn.(driver.QueryerContext); ok {
		started := time.Now()
		rows, err := conn.QueryContext(ctx, query, args)
		if err != nil {
			traceSQL(ctx, query, "query", time.Since(started), err)
			return nil, err
		}
		return &tracedSQLiteRows{Rows: rows, query: query, started: started, ctx: ctx}, nil
	}
	return nil, driver.ErrSkip
}

type tracedSQLiteStmt struct {
	driver.Stmt
	query string
}

func (s tracedSQLiteStmt) Exec(args []driver.Value) (driver.Result, error) {
	started := time.Now()
	result, err := s.Stmt.Exec(args)
	traceSQL(context.Background(), s.query, "exec", time.Since(started), err)
	return result, err
}

func (s tracedSQLiteStmt) Query(args []driver.Value) (driver.Rows, error) {
	started := time.Now()
	rows, err := s.Stmt.Query(args)
	if err != nil {
		traceSQL(context.Background(), s.query, "query", time.Since(started), err)
		return nil, err
	}
	return &tracedSQLiteRows{Rows: rows, query: s.query, started: started, ctx: context.Background()}, nil
}

func (s tracedSQLiteStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if stmt, ok := s.Stmt.(driver.StmtExecContext); ok {
		started := time.Now()
		result, err := stmt.ExecContext(ctx, args)
		traceSQL(ctx, s.query, "exec", time.Since(started), err)
		return result, err
	}
	return nil, driver.ErrSkip
}

func (s tracedSQLiteStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if stmt, ok := s.Stmt.(driver.StmtQueryContext); ok {
		started := time.Now()
		rows, err := stmt.QueryContext(ctx, args)
		if err != nil {
			traceSQL(ctx, s.query, "query", time.Since(started), err)
			return nil, err
		}
		return &tracedSQLiteRows{Rows: rows, query: s.query, started: started, ctx: ctx}, nil
	}
	return nil, driver.ErrSkip
}

type tracedSQLiteRows struct {
	driver.Rows
	query    string
	started  time.Time
	ctx      context.Context
	done     bool
	rowCount int
	firstRow time.Duration
}

func (r *tracedSQLiteRows) Next(dest []driver.Value) error {
	err := r.Rows.Next(dest)
	if err == nil {
		r.rowCount++
		if r.rowCount == 1 {
			r.firstRow = time.Since(r.started)
		}
	}
	if err == io.EOF || err != nil {
		r.finish(err)
	}
	return err
}

func (r *tracedSQLiteRows) Close() error {
	err := r.Rows.Close()
	r.finish(err)
	return err
}

func (r *tracedSQLiteRows) finish(err error) {
	if r.done {
		return
	}
	r.done = true
	if err == io.EOF {
		err = nil
	}
	fields := map[string]any{
		"operation": "query", "duration_ns": time.Since(r.started).Nanoseconds(),
		"sql": strings.Join(strings.Fields(r.query), " "), "row_count": r.rowCount,
		"first_row_ns": r.firstRow.Nanoseconds(),
	}
	if err != nil {
		fields["failed"] = true
		fields["error_type"] = fmt.Sprintf("%T", err)
	}
	traceEvent(r.ctx, "sqlite", fields)
}

func traceSQL(ctx context.Context, query, operation string, duration time.Duration, err error) {
	fields := map[string]any{"operation": operation, "duration_ns": duration.Nanoseconds(), "sql": strings.Join(strings.Fields(query), " ")}
	if err != nil {
		fields["failed"] = true
		fields["error_type"] = fmt.Sprintf("%T", err)
	}
	traceEvent(ctx, "sqlite", fields)
}

func openUserSQLite(dsn string) (*sql.DB, error) {
	if traceEnabled {
		return sql.OpenDB(tracedSQLiteConnector{driver: tracedSQLiteDriver{base: &modernsqlite.Driver{}}, dsn: dsn}), nil
	}
	return sql.Open("sqlite", dsn)
}

type tracedSQLiteConnector struct {
	driver driver.Driver
	dsn    string
}

func (c tracedSQLiteConnector) Connect(context.Context) (driver.Conn, error) {
	return c.driver.Open(c.dsn)
}
func (c tracedSQLiteConnector) Driver() driver.Driver { return c.driver }
