package dbconnpool

// import (
// 	"fmt"
// 	"sync"
// 	"sync/atomic"
// 	"testing"
// 	"time"
// )

// // go test -v ./... 2>&1 | grep -E "^--- (PASS|FAIL)" | sort | uniq -c
// func TestInit(t *testing.T) {
// 	t.Log("=== Testing Init ===")

// 	t.Logf("initializing pool with MinConn=%d, MaxConn=%d", testConfig.MinConn, testConfig.MaxConn)
// 	pool, err := Init(testConfig)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("pool created: totalConn=%d, idle=%d, active=%d",
// 		pool.totalConn, len(pool.idelConn), len(pool.activeConn))

// 	if len(pool.activeConn) != 0 {
// 		t.Fatal("expected no active conns on init")
// 	}
// 	if pool.totalConn != pool.config.MinConn {
// 		t.Fatalf("expected %d total conns, got %d", pool.config.MinConn, pool.totalConn)
// 	}
// 	t.Log("init OK: pool warmed up to MinConn with zero active connections")
// }

// func TestGetConnection(t *testing.T) {
// 	t.Log("=== Testing GetConnection ===")

// 	pool, err := Init(testConfig)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("pool ready: idle=%d", len(pool.idelConn))

// 	t.Log("acquiring one connection...")
// 	conn, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("acquired: active=%d, idle=%d", len(pool.activeConn), len(pool.idelConn))

// 	if len(pool.activeConn) != 1 {
// 		t.Fatalf("expected 1 active conn, got %d", len(pool.activeConn))
// 	}
// 	if len(pool.idelConn) != int(testConfig.MinConn)-1 {
// 		t.Fatalf("expected %d idle conns, got %d", testConfig.MinConn-1, len(pool.idelConn))
// 	}
// 	t.Log("get connection OK: connection moved from idle to active")
// 	_ = conn
// }

// func TestRelease(t *testing.T) {
// 	t.Log("=== Testing Release ===")

// 	pool, err := Init(testConfig)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	t.Log("acquiring connection...")
// 	conn, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("before release: active=%d, idle=%d", len(pool.activeConn), len(pool.idelConn))

// 	t.Log("releasing connection...")
// 	conn.Release()
// 	t.Logf("after release: active=%d, idle=%d", len(pool.activeConn), len(pool.idelConn))

// 	if len(pool.activeConn) != 0 {
// 		t.Fatal("expected no active conns after release")
// 	}
// 	if len(pool.idelConn) != int(testConfig.MinConn) {
// 		t.Fatalf("expected %d idle conns after release, got %d", testConfig.MinConn, len(pool.idelConn))
// 	}
// 	t.Log("release OK: connection returned to idle pool")
// }

// func TestMaxConnection(t *testing.T) {
// 	t.Log("=== Testing MaxConnection ===")

// 	cfg := testConfig
// 	cfg.MinConn = 1
// 	cfg.MaxConn = 2
// 	t.Logf("config: MinConn=%d, MaxConn=%d", cfg.MinConn, cfg.MaxConn)

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	t.Log("acquiring connection 1...")
// 	conn1, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("conn1 acquired: totalConn=%d", pool.totalConn)

// 	t.Log("acquiring connection 2...")
// 	conn2, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("conn2 acquired: totalConn=%d", pool.totalConn)

// 	t.Log("attempting to create connection beyond MaxConn (should fail)...")
// 	_, err = pool.createConnect()
// 	if err != ErrMaxConnection {
// 		t.Fatalf("expected ErrMaxConnection, got %v", err)
// 	}
// 	t.Logf("got expected error: %v", err)
// 	t.Log("max connection OK: pool refused to exceed MaxConn")

// 	_ = conn1
// 	_ = conn2
// }

// func TestConcurrentGetConnection(t *testing.T) {
// 	t.Log("=== Testing Concurrent GetConnection ===")

// 	cfg := testConfig
// 	cfg.MinConn = 1
// 	cfg.MaxConn = 2

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	var wg sync.WaitGroup
// 	numGoroutines := 4
// 	errors := make(chan error, numGoroutines)

// 	t.Logf("launching %d goroutines against pool (MinConn=%d, MaxConn=%d)",
// 		numGoroutines, cfg.MinConn, cfg.MaxConn)

// 	for i := 0; i < numGoroutines; i++ {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()
// 			conn, err := pool.GetConnetion()
// 			if err != nil {
// 				errors <- err
// 				return
// 			}
// 			t.Logf("goroutine %d: acquired connection", id)
// 			// Simulate some work
// 			time.Sleep(10 * time.Millisecond)
// 			conn.Release()
// 			t.Logf("goroutine %d: released connection", id)
// 		}(i)
// 	}

// 	wg.Wait()
// 	close(errors)
// 	t.Log("all goroutines finished")

// 	for err := range errors {
// 		t.Errorf("goroutine error: %v", err)
// 	}

// 	t.Logf("final state: active=%d, idle=%d, totalConn=%d",
// 		len(pool.activeConn), len(pool.idelConn), pool.totalConn)

// 	// After all goroutines done, nothing should be active
// 	if len(pool.activeConn) != 0 {
// 		t.Fatalf("expected 0 active conns after all released, got %d", len(pool.activeConn))
// 	}
// 	t.Log("concurrent get OK: all connections released cleanly")
// }

// // All goroutines fight for a very limited pool
// func TestConcurrentGetConnectionUnderPressure(t *testing.T) {
// 	t.Log("=== Testing Concurrent GetConnection Under Pressure ===")

// 	cfg := testConfig
// 	cfg.MinConn = 1
// 	cfg.MaxConn = 2

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	var wg sync.WaitGroup
// 	numGoroutines := 50
// 	successCount := atomic.Int32{}
// 	errorCount := atomic.Int32{}

// 	t.Logf("launching %d goroutines against tiny pool (MaxConn=%d) — contention expected",
// 		numGoroutines, cfg.MaxConn)

// 	for i := 0; i < numGoroutines; i++ {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()

// 			idle, active, total := len(pool.idelConn), len(pool.activeConn), pool.totalConn
// 			fmt.Printf("[goroutine %d] before acquire: idle=%d active=%d total=%d\n", id, idle, active, total)

// 			conn, err := pool.GetConnetion()

// 			idle, active, total = len(pool.idelConn), len(pool.activeConn), pool.totalConn
// 			fmt.Printf("[goroutine %d] after acquire:  idle=%d active=%d total=%d (err=%v)\n", id, idle, active, total, err)

// 			if err != nil {
// 				t.Logf("goroutine %d: failed to acquire (%v)", id, err)
// 				errorCount.Add(1)
// 				return
// 			}
// 			t.Logf("goroutine %d: acquired connection %p", id, conn)
// 			successCount.Add(1)
// 			time.Sleep(20 * time.Millisecond)

// 			fmt.Printf("[goroutine %d] releasing connection %p\n", id, conn)
// 			conn.Release()

// 			idle, active, total = len(pool.idelConn), len(pool.activeConn), pool.totalConn
// 			fmt.Printf("[goroutine %d] after release:  idle=%d active=%d total=%d\n", id, idle, active, total)
// 		}(i)
// 	}

// 	wg.Wait()

// 	t.Logf("results — success: %d, errors: %d", successCount.Load(), errorCount.Load())
// 	t.Logf("final state: active=%d, idle=%d, totalConn=%d",
// 		len(pool.activeConn), len(pool.idelConn), pool.totalConn)

// 	if len(pool.activeConn) != 0 {
// 		t.Fatalf("expected 0 active after all done, got %d", len(pool.activeConn))
// 	}
// 	t.Log("pressure test OK: pool consistent after heavy contention")
// }

// func TestConcurrentNeverExceedsMaxConn(t *testing.T) {
// 	t.Log("=== Testing Concurrent Never Exceeds MaxConn ===")

// 	cfg := testConfig
// 	cfg.MinConn = 1
// 	cfg.MaxConn = 2

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	var wg sync.WaitGroup
// 	numGoroutines := 30

// 	t.Logf("launching %d goroutines, watching that totalConn never passes %d",
// 		numGoroutines, cfg.MaxConn)

// 	for i := 0; i < numGoroutines; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			conn, err := pool.GetConnetion()
// 			if err != nil {
// 				return
// 			}
// 			// Check mid-flight that we never exceed max
// 			if pool.totalConn > cfg.MaxConn {
// 				t.Errorf("totalConn %d exceeded MaxConn %d", pool.totalConn, cfg.MaxConn)
// 			}
// 			time.Sleep(5 * time.Millisecond)
// 			conn.Release()
// 		}()
// 	}

// 	wg.Wait()

// 	t.Logf("final totalConn=%d (limit %d)", pool.totalConn, cfg.MaxConn)

// 	if pool.totalConn > cfg.MaxConn {
// 		t.Fatalf("final totalConn %d exceeded MaxConn %d", pool.totalConn, cfg.MaxConn)
// 	}
// 	t.Log("max conn invariant OK: limit was never exceeded")
// }

// func TestReleasedConnectionIsReused(t *testing.T) {
// 	t.Log("=== Testing Released Connection Is Reused ===")

// 	cfg := testConfig
// 	cfg.MinConn = 1
// 	cfg.MaxConn = 1 // pool of exactly one: reuse is the only legal behavior

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	t.Log("acquiring connection for the first time...")
// 	conn1, err := pool.GetConnetion()
// 	if err != nil {
// 		fmt.Printf("Error while getting conn")
// 		t.Fatal(err)
// 	}
// 	t.Logf("first acquire:  conn at %p, totalConn=%d", conn1, pool.totalConn)

// 	t.Log("releasing it back to the pool...")
// 	conn1.Release()

// 	t.Log("acquiring again — should get the SAME connection back...")
// 	conn2, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("second acquire: conn at %p, totalConn=%d", conn2, pool.totalConn)

// 	if conn1 != conn2 {
// 		t.Fatalf("pool created a new connection instead of reusing: first=%p, second=%p", conn1, conn2)
// 	}
// 	if pool.totalConn != 1 {
// 		t.Fatalf("expected totalConn to stay at 1, got %d", pool.totalConn)
// 	}
// 	t.Log("reuse OK: same pointer returned, no new connection created")
// }

// func TestConcurrentReuseNeverCreatesExtraConns(t *testing.T) {
// 	t.Log("=== Testing Concurrent Reuse (distinct pointers <= MaxConn) ===")

// 	cfg := testConfig
// 	cfg.MinConn = 1
// 	cfg.MaxConn = 3

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	var (
// 		wg        sync.WaitGroup
// 		mu        sync.Mutex
// 		seenConns = make(map[*Conn]struct{}) // every distinct connection ever handed out
// 	)
// 	numGoroutines := 30

// 	t.Logf("launching %d goroutines against MaxConn=%d, tracking unique connection addresses",
// 		numGoroutines, cfg.MaxConn)

// 	for i := 0; i < numGoroutines; i++ {
// 		wg.Add(1)
// 		go func(id int) {
// 			defer wg.Done()
// 			conn, err := pool.GetConnetion()
// 			if err != nil {
// 				return
// 			}
// 			mu.Lock()
// 			if _, ok := seenConns[conn]; !ok {
// 				seenConns[conn] = struct{}{}
// 				t.Logf("goroutine %d: got NEW connection %p (distinct so far: %d)", id, conn, len(seenConns))
// 			} else {
// 				t.Logf("goroutine %d: got REUSED connection %p", id, conn)
// 			}
// 			mu.Unlock()

// 			time.Sleep(5 * time.Millisecond)
// 			conn.Release()
// 		}(i)
// 	}

// 	wg.Wait()

// 	t.Logf("distinct connections handed out: %d (MaxConn=%d)", len(seenConns), cfg.MaxConn)

// 	if len(seenConns) > int(cfg.MaxConn) {
// 		t.Fatalf("pool handed out %d distinct connections, exceeding MaxConn=%d — released connections are not being reused",
// 			len(seenConns), cfg.MaxConn)
// 	}
// 	t.Log("concurrent reuse OK: connection objects were recycled, never over-created")
// }

// func TestReleaseUnhealthy(t *testing.T) {
// 	t.Log("=== Testing Release Unhealthy Connection ===")

// 	pool, err := Init(testConfig)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	conn, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("acquired: active=%d, idle=%d, totalConn=%d",
// 		len(pool.activeConn), len(pool.idelConn), pool.totalConn)

// 	// Simulate a broken connection by closing the underlying socket.
// 	conn.netConn.Close()

// 	idleBefore := len(pool.idelConn)
// 	totalBefore := pool.totalConn

// 	conn.Release()

// 	t.Logf("after unhealthy release: active=%d, idle=%d, totalConn=%d",
// 		len(pool.activeConn), len(pool.idelConn), pool.totalConn)

// 	if len(pool.activeConn) != 0 {
// 		t.Fatal("expected 0 active conns after unhealthy release")
// 	}
// 	if len(pool.idelConn) != idleBefore {
// 		t.Fatalf("idle count should not grow after unhealthy release: before=%d after=%d",
// 			idleBefore, len(pool.idelConn))
// 	}
// 	if pool.totalConn != totalBefore-1 {
// 		t.Fatalf("expected totalConn to decrement by 1, before=%d after=%d",
// 			totalBefore, pool.totalConn)
// 	}
// 	t.Log("unhealthy release OK: conn discarded, not returned to idle")
// }

// func TestIsAlive(t *testing.T) {
// 	t.Log("=== Testing isAlive ===")

// 	pool, err := Init(testConfig)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	conn, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer conn.Release()

// 	if !conn.isAlive() {
// 		t.Fatal("expected isAlive()=true for a fresh connection")
// 	}
// 	t.Log("isAlive true: OK")

// 	// Close the socket and verify the method detects it.
// 	conn.netConn.Close()
// 	if conn.isAlive() {
// 		t.Fatal("expected isAlive()=false after NetConn.Close()")
// 	}
// 	t.Log("isAlive false after close: OK")
// }

// func TestAddTimeoutBug(t *testing.T) {
// 	t.Log("=== Testing addTimeOuts respects config duration ===")

// 	cfg := testConfig
// 	cfg.IdealConnTimeOut = 30 // Init() multiplies by time.Second → 30s

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	conn, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer conn.Release()

// 	expectedTimeout := time.Now().Add(30 * time.Second)
// 	diff := conn.timeOut.Sub(expectedTimeout)
// 	if diff < 0 {
// 		diff = -diff
// 	}

// 	t.Logf("conn.timeOut=%v, expected≈%v, diff=%v", conn.timeOut, expectedTimeout, diff)

// 	if diff > 500*time.Millisecond {
// 		t.Fatalf("addTimeOuts ignores its duration arg: expected timeout ~30s from now but got diff=%v\n"+
// 			"Fix: change `time.Now().Add(time.Second)` to `time.Now().Add(t)` in addTimeOuts()", diff)
// 	}
// 	t.Log("addTimeOuts duration OK")
// }

// func TestTimeoutSweeper(t *testing.T) {
// 	t.Log("=== Testing listenToTimeOuts sweeper ===")

// 	cfg := testConfig
// 	cfg.IdealConnTimeOut = 1 // 1s after Init() multiplier

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	conn, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("acquired conn: active=%d", len(pool.activeConn))

// 	// Force the conn's timeOut into the past so the sweeper sees it as expired.
// 	conn.timeOut = time.Now().Add(-2 * time.Second)

// 	// Wait for at least one sweep tick (interval = IdealConnTimeOut/2 = 500ms).
// 	// Sleep a bit longer to avoid flakiness.
// 	time.Sleep(800 * time.Millisecond)

// 	t.Logf("after sweep window: active=%d, idle=%d", len(pool.activeConn), len(pool.idelConn))

// 	pool.mutx.Lock()
// 	activeCount := len(pool.activeConn)
// 	pool.mutx.Unlock()

// 	if activeCount != 0 {
// 		t.Fatalf("sweeper did not evict expired connection: active=%d", activeCount)
// 	}
// 	t.Log("timeout sweeper OK: expired connection evicted from active")
// }

// func TestDoubleRelease(t *testing.T) {
// 	t.Log("=== Testing Double Release ===")

// 	cfg := testConfig
// 	cfg.MaxConn = 5

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	conn, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	conn.Release()
// 	idleAfterFirst := len(pool.idelConn)
// 	t.Logf("after first release: idle=%d", idleAfterFirst)

// 	// Second Release() — should not panic and should not inflate idle count.
// 	defer func() {
// 		if r := recover(); r != nil {
// 			t.Fatalf("second Release() panicked: %v", r)
// 		}
// 	}()
// 	conn.Release()

// 	idleAfterSecond := len(pool.idelConn)
// 	t.Logf("after second release: idle=%d", idleAfterSecond)

// 	if idleAfterSecond > idleAfterFirst {
// 		t.Fatalf("double release inflated idle pool: after first=%d after second=%d",
// 			idleAfterFirst, idleAfterSecond)
// 	}
// 	t.Log("double release OK: no panic and idle pool not corrupted")
// }

// func TestGetConnectionBlocksUntilReleased(t *testing.T) {
// 	t.Log("=== Testing GetConnection blocks until a connection is released ===")

// 	cfg := testConfig
// 	cfg.MinConn = 1
// 	cfg.MaxConn = 1

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	// Hold the only connection.
// 	first, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Log("first connection acquired, pool exhausted")

// 	var (
// 		second    *Conn
// 		secondErr error
// 		wg        sync.WaitGroup
// 	)

// 	wg.Add(1)
// 	go func() {
// 		defer wg.Done()
// 		second, secondErr = pool.GetConnetion() // should block
// 	}()

// 	// Give the goroutine time to block on the idle channel.
// 	time.Sleep(50 * time.Millisecond)

// 	t.Log("releasing first connection...")
// 	first.Release()

// 	// Now the goroutine should unblock within a reasonable time.
// 	done := make(chan struct{})
// 	go func() { wg.Wait(); close(done) }()

// 	select {
// 	case <-done:
// 	case <-time.After(2 * time.Second):
// 		t.Fatal("GetConnetion() did not unblock after Release() — possible deadlock")
// 	}

// 	if secondErr != nil {
// 		t.Fatalf("second GetConnetion() returned error: %v", secondErr)
// 	}
// 	if second == nil {
// 		t.Fatal("second GetConnetion() returned nil connection")
// 	}
// 	t.Logf("second connection acquired at %p", second)
// 	second.Release()
// 	t.Log("blocking GetConnetion OK: unblocked after release")
// }

// func TestTotalConnAfterCloseAndReacquire(t *testing.T) {
// 	t.Log("=== Testing totalConn after closeConn then reacquire ===")

// 	cfg := testConfig
// 	cfg.MinConn = 1
// 	cfg.MaxConn = 1

// 	pool, err := Init(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	conn, err := pool.GetConnetion()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("acquired: totalConn=%d", pool.totalConn)

// 	// Close directly — this should decrement totalConn.
// 	pool.removeConnFromActive(conn)
// 	pool.closeConn(conn)
// 	t.Logf("after closeConn: totalConn=%d", pool.totalConn)

// 	if pool.totalConn != 0 {
// 		t.Fatalf("expected totalConn=0 after closeConn, got %d", pool.totalConn)
// 	}

// 	// Now we should be able to create a new connection.
// 	newConn, err := pool.createConnect()
// 	if err != nil {
// 		t.Fatalf("createConnect() failed after closeConn: %v", err)
// 	}
// 	t.Logf("new connection created: totalConn=%d", pool.totalConn)

// 	if pool.totalConn != 1 {
// 		t.Fatalf("expected totalConn=1 after reacquire, got %d", pool.totalConn)
// 	}
// 	pool.idelConn <- newConn
// 	t.Log("totalConn decrement/reacquire OK")
// }
