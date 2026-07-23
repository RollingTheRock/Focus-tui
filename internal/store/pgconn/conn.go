package pgconn

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/RollingTheRock/Focus-tui/internal/platform"

	embedded "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EmbeddedPostgres wraps an embedded PostgreSQL instance and its connection pool.
type EmbeddedPostgres struct {
	db     *embedded.EmbeddedPostgres
	pool   *pgxpool.Pool
	port   uint32
	dataDir string
}

// Start launches an embedded PostgreSQL instance and returns a connection pool.
// dataDir is where PostgreSQL data files will live (should be persistent across restarts).
func Start(ctx context.Context, dataDir string) (*EmbeddedPostgres, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create pg data dir: %w", err)
	}

	// Find a free port to avoid conflicts with system PostgreSQL.
	port, err := getFreePort()
	if err != nil {
		return nil, fmt.Errorf("get free port: %w", err)
	}

	cfg := embedded.DefaultConfig().
		Username("focus").
		Password("focus").
		Database("focus").
		Port(uint32(port)).
		DataPath(filepath.Join(dataDir, "data")).
		RuntimePath(filepath.Join(dataDir, "runtime")).
		CachePath(filepath.Join(dataDir, "cache")).
		StartTimeout(30 * time.Second).
		Logger(nil)

	ep := embedded.NewDatabase(cfg)
	if err := ep.Start(); err != nil {
		return nil, fmt.Errorf("start embedded postgres: %w", err)
	}

	connStr := fmt.Sprintf("postgres://focus:focus@localhost:%d/focus?sslmode=disable", port)
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		_ = ep.Stop()
		return nil, fmt.Errorf("connect to embedded postgres: %w", err)
	}

	// Verify connectivity.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		_ = ep.Stop()
		return nil, fmt.Errorf("ping embedded postgres: %w", err)
	}

	return &EmbeddedPostgres{
		db:      ep,
		pool:    pool,
		port:    uint32(port),
		dataDir: dataDir,
	}, nil
}

// Pool returns the connection pool.
func (ep *EmbeddedPostgres) Pool() *pgxpool.Pool {
	return ep.pool
}

// Port returns the listening port.
func (ep *EmbeddedPostgres) Port() uint32 {
	return ep.port
}

// Stop gracefully shuts down the embedded PostgreSQL instance.
func (ep *EmbeddedPostgres) Stop() error {
	if ep.pool != nil {
		ep.pool.Close()
	}
	if ep.db != nil {
		return ep.db.Stop()
	}
	return nil
}

// ExternalPool creates a connection pool for an external PostgreSQL instance.
func ExternalPool(ctx context.Context, connStr string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("connect to external postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping external postgres: %w", err)
	}
	return pool, nil
}

// DefaultDataDir returns the platform-appropriate postgres data directory.
func DefaultDataDir() (string, error) {
	dir, err := platform.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "postgres"), nil
}

// getFreePort asks the OS for a free TCP port.
func getFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
