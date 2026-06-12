package dbconnpool

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	t.Log("=== Testing Init ===")

	t.Logf("initializing pool with MinConn=%d, MaxConn=%d", testConfig.MinConn, testConfig.MaxConn)
	pool, err := Init(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pool created: totalConn=%d, idle=%d, active=%d",
		pool.totalConn, len(pool.idelConn), len(pool.activeConn))

	if len(pool.activeConn) != 0 {
		t.Fatal("expected no active conns on init")
	}
	if pool.totalConn != pool.config.MinConn {
		t.Fatalf("expected %d total conns, got %d", pool.config.MinConn, pool.totalConn)
	}
	t.Log("init OK: pool warmed up to MinConn with zero active connections")
}

func TestGetConnection(t *testing.T) {
	t.Log("=== Testing GetConnection ===")

	pool, err := Init(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pool ready: idle=%d", len(pool.idelConn))

	t.Log("acquiring one connection...")
	conn, err := pool.GetConnetion()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("acquired: active=%d, idle=%d", len(pool.activeConn), len(pool.idelConn))

	if len(pool.activeConn) != 1 {
		t.Fatalf("expected 1 active conn, got %d", len(pool.activeConn))
	}
	if len(pool.idelConn) != int(testConfig.MinConn)-1 {
		t.Fatalf("expected %d idle conns, got %d", testConfig.MinConn-1, len(pool.idelConn))
	}
	t.Log("get connection OK: connection moved from idle to active")
	_ = conn
}

func TestRelease(t *testing.T) {
	t.Log("=== Testing Release ===")

	pool, err := Init(testConfig)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("acquiring connection...")
	conn, err := pool.GetConnetion()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("before release: active=%d, idle=%d", len(pool.activeConn), len(pool.idelConn))

	t.Log("releasing connection...")
	conn.Release()
	t.Logf("after release: active=%d, idle=%d", len(pool.activeConn), len(pool.idelConn))

	if len(pool.activeConn) != 0 {
		t.Fatal("expected no active conns after release")
	}
	if len(pool.idelConn) != int(testConfig.MinConn) {
		t.Fatalf("expected %d idle conns after release, got %d", testConfig.MinConn, len(pool.idelConn))
	}
	t.Log("release OK: connection returned to idle pool")
}

func TestMaxConnection(t *testing.T) {
	t.Log("=== Testing MaxConnection ===")

	cfg := testConfig
	cfg.MinConn = 1
	cfg.MaxConn = 2
	t.Logf("config: MinConn=%d, MaxConn=%d", cfg.MinConn, cfg.MaxConn)

	pool, err := Init(cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("acquiring connection 1...")
	conn1, err := pool.GetConnetion()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("conn1 acquired: totalConn=%d", pool.totalConn)

	t.Log("acquiring connection 2...")
	conn2, err := pool.GetConnetion()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("conn2 acquired: totalConn=%d", pool.totalConn)

	t.Log("attempting to create connection beyond MaxConn (should fail)...")
	_, err = pool.createConnect()
	if err != ErrMaxConnection {
		t.Fatalf("expected ErrMaxConnection, got %v", err)
	}
	t.Logf("got expected error: %v", err)
	t.Log("max connection OK: pool refused to exceed MaxConn")

	_ = conn1
	_ = conn2
}

func TestConcurrentGetConnection(t *testing.T) {
	t.Log("=== Testing Concurrent GetConnection ===")

	cfg := testConfig
	cfg.MinConn = 1
	cfg.MaxConn = 2

	pool, err := Init(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	numGoroutines := 4
	errors := make(chan error, numGoroutines)

	t.Logf("launching %d goroutines against pool (MinConn=%d, MaxConn=%d)",
		numGoroutines, cfg.MinConn, cfg.MaxConn)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := pool.GetConnetion()
			if err != nil {
				errors <- err
				return
			}
			t.Logf("goroutine %d: acquired connection", id)
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			conn.Release()
			t.Logf("goroutine %d: released connection", id)
		}(i)
	}

	wg.Wait()
	close(errors)
	t.Log("all goroutines finished")

	for err := range errors {
		t.Errorf("goroutine error: %v", err)
	}

	t.Logf("final state: active=%d, idle=%d, totalConn=%d",
		len(pool.activeConn), len(pool.idelConn), pool.totalConn)

	// After all goroutines done, nothing should be active
	if len(pool.activeConn) != 0 {
		t.Fatalf("expected 0 active conns after all released, got %d", len(pool.activeConn))
	}
	t.Log("concurrent get OK: all connections released cleanly")
}

// All goroutines fight for a very limited pool
func TestConcurrentGetConnectionUnderPressure(t *testing.T) {
	t.Log("=== Testing Concurrent GetConnection Under Pressure ===")

	cfg := testConfig
	cfg.MinConn = 1
	cfg.MaxConn = 2

	pool, err := Init(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	successCount := atomic.Int32{}
	errorCount := atomic.Int32{}

	t.Logf("launching %d goroutines against tiny pool (MaxConn=%d) — contention expected",
		numGoroutines, cfg.MaxConn)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			idle, active, total := len(pool.idelConn), len(pool.activeConn), pool.totalConn
			fmt.Printf("[goroutine %d] before acquire: idle=%d active=%d total=%d\n", id, idle, active, total)

			conn, err := pool.GetConnetion()

			idle, active, total = len(pool.idelConn), len(pool.activeConn), pool.totalConn
			fmt.Printf("[goroutine %d] after acquire:  idle=%d active=%d total=%d (err=%v)\n", id, idle, active, total, err)

			if err != nil {
				t.Logf("goroutine %d: failed to acquire (%v)", id, err)
				errorCount.Add(1)
				return
			}
			t.Logf("goroutine %d: acquired connection %p", id, conn)
			successCount.Add(1)
			time.Sleep(20 * time.Millisecond)

			fmt.Printf("[goroutine %d] releasing connection %p\n", id, conn)
			conn.Release()

			idle, active, total = len(pool.idelConn), len(pool.activeConn), pool.totalConn
			fmt.Printf("[goroutine %d] after release:  idle=%d active=%d total=%d\n", id, idle, active, total)
		}(i)
	}

	wg.Wait()

	t.Logf("results — success: %d, errors: %d", successCount.Load(), errorCount.Load())
	t.Logf("final state: active=%d, idle=%d, totalConn=%d",
		len(pool.activeConn), len(pool.idelConn), pool.totalConn)

	if len(pool.activeConn) != 0 {
		t.Fatalf("expected 0 active after all done, got %d", len(pool.activeConn))
	}
	t.Log("pressure test OK: pool consistent after heavy contention")
}

func TestConcurrentNeverExceedsMaxConn(t *testing.T) {
	t.Log("=== Testing Concurrent Never Exceeds MaxConn ===")

	cfg := testConfig
	cfg.MinConn = 1
	cfg.MaxConn = 2

	pool, err := Init(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	numGoroutines := 30

	t.Logf("launching %d goroutines, watching that totalConn never passes %d",
		numGoroutines, cfg.MaxConn)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := pool.GetConnetion()
			if err != nil {
				return
			}
			// Check mid-flight that we never exceed max
			if pool.totalConn > cfg.MaxConn {
				t.Errorf("totalConn %d exceeded MaxConn %d", pool.totalConn, cfg.MaxConn)
			}
			time.Sleep(5 * time.Millisecond)
			conn.Release()
		}()
	}

	wg.Wait()

	t.Logf("final totalConn=%d (limit %d)", pool.totalConn, cfg.MaxConn)

	if pool.totalConn > cfg.MaxConn {
		t.Fatalf("final totalConn %d exceeded MaxConn %d", pool.totalConn, cfg.MaxConn)
	}
	t.Log("max conn invariant OK: limit was never exceeded")
}

func TestReleasedConnectionIsReused(t *testing.T) {
	t.Log("=== Testing Released Connection Is Reused ===")

	cfg := testConfig
	cfg.MinConn = 1
	cfg.MaxConn = 1 // pool of exactly one: reuse is the only legal behavior

	pool, err := Init(cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("acquiring connection for the first time...")
	conn1, err := pool.GetConnetion()
	if err != nil {
		fmt.Printf("Error while getting conn")
		t.Fatal(err)
	}
	t.Logf("first acquire:  conn at %p, totalConn=%d", conn1, pool.totalConn)

	t.Log("releasing it back to the pool...")
	conn1.Release()

	t.Log("acquiring again — should get the SAME connection back...")
	conn2, err := pool.GetConnetion()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("second acquire: conn at %p, totalConn=%d", conn2, pool.totalConn)

	if conn1 != conn2 {
		t.Fatalf("pool created a new connection instead of reusing: first=%p, second=%p", conn1, conn2)
	}
	if pool.totalConn != 1 {
		t.Fatalf("expected totalConn to stay at 1, got %d", pool.totalConn)
	}
	t.Log("reuse OK: same pointer returned, no new connection created")
}

func TestConcurrentReuseNeverCreatesExtraConns(t *testing.T) {
	t.Log("=== Testing Concurrent Reuse (distinct pointers <= MaxConn) ===")

	cfg := testConfig
	cfg.MinConn = 1
	cfg.MaxConn = 3

	pool, err := Init(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		seenConns = make(map[*Conn]struct{}) // every distinct connection ever handed out
	)
	numGoroutines := 30

	t.Logf("launching %d goroutines against MaxConn=%d, tracking unique connection addresses",
		numGoroutines, cfg.MaxConn)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := pool.GetConnetion()
			if err != nil {
				return
			}
			mu.Lock()
			if _, ok := seenConns[conn]; !ok {
				seenConns[conn] = struct{}{}
				t.Logf("goroutine %d: got NEW connection %p (distinct so far: %d)", id, conn, len(seenConns))
			} else {
				t.Logf("goroutine %d: got REUSED connection %p", id, conn)
			}
			mu.Unlock()

			time.Sleep(5 * time.Millisecond)
			conn.Release()
		}(i)
	}

	wg.Wait()

	t.Logf("distinct connections handed out: %d (MaxConn=%d)", len(seenConns), cfg.MaxConn)

	if len(seenConns) > int(cfg.MaxConn) {
		t.Fatalf("pool handed out %d distinct connections, exceeding MaxConn=%d — released connections are not being reused",
			len(seenConns), cfg.MaxConn)
	}
	t.Log("concurrent reuse OK: connection objects were recycled, never over-created")
}
