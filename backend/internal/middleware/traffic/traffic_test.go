package traffic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/middleware/ratelimit"
	"my_feed_system/internal/response"
)

type stubChecker struct {
	mu     sync.Mutex
	counts map[string]int64
	err    error
}

func (s *stubChecker) Allow(_ context.Context, scope string, subject string, limit int64, _ time.Duration) (ratelimit.Result, error) {
	if s.err != nil {
		return ratelimit.Result{}, s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.counts == nil {
		s.counts = map[string]int64{}
	}
	key := scope + ":" + subject
	s.counts[key]++
	count := s.counts[key]
	allowed := limit <= 0 || count <= limit
	retry := time.Duration(0)
	if !allowed {
		retry = time.Second
	}
	return ratelimit.Result{Allowed: allowed, Count: count, Remaining: max64(0, limit-count), RetryAfter: retry}, nil
}

func max64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

type memoryBox struct {
	mu        sync.Mutex
	denies    map[string]int64
	penalties map[string]struct{}
}

func newMemoryBox() *memoryBox {
	return &memoryBox{denies: map[string]int64{}, penalties: map[string]struct{}{}}
}

func (b *memoryBox) key(dimension string, subject string) string {
	return dimension + ":" + subject
}

func (b *memoryBox) Denied(_ context.Context, dimension string, subject string, _ time.Duration) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := b.key(dimension, subject)
	b.denies[k]++
	return b.denies[k], nil
}

func (b *memoryBox) HasPenalty(_ context.Context, dimension string, subject string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.penalties[b.key(dimension, subject)]
	return ok, nil
}

func (b *memoryBox) ApplyPenalty(_ context.Context, dimension string, subject string, _ time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.penalties[b.key(dimension, subject)] = struct{}{}
	return nil
}

func testConfig() Config {
	return Config{
		IPLimit:             3,
		AccountLimit:        5,
		Window:              time.Minute,
		PenaltyIPLimit:      1,
		PenaltyAccountLimit: 1,
		DenyThreshold:       2,
		DenyWindow:          time.Minute,
		PenaltyTTL:          time.Minute,
		MaxInFlight:         0,
		Timeout:             50 * time.Millisecond,
		FailOpen:            true,
		SkipPaths:           []string{"/ping", "/metrics", "/wallet/stripe/notify"},
		SkipPrefixes:        []string{"/static/"},
	}
}

func testRouter(g *Guard) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(g.Middleware())
	r.POST("/feed/listRecommend", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	r.POST("/wallet/stripe/notify", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/static/videos/a.mp4", func(c *gin.Context) { c.String(http.StatusOK, "media") })
	return r
}

func doPOST(r http.Handler, path string, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.RemoteAddr = ip + ":1234"
	req.Header.Set("X-Real-IP", ip)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGlobalIPLimitReturns429(t *testing.T) {
	checker := &stubChecker{}
	g := newGuard(checker, newMemoryBox(), testConfig())
	r := testRouter(g)

	var last *httptest.ResponseRecorder
	for i := 0; i < 4; i++ {
		last = doPOST(r, "/feed/listRecommend", "1.1.1.1")
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", last.Code, last.Body.String())
	}
	if last.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	var body response.Body
	if err := json.Unmarshal(last.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != response.RateLimited {
		t.Fatalf("code=%s", body.Code)
	}
}

func TestSkipPathsAndStaticPrefix(t *testing.T) {
	checker := &stubChecker{}
	cfg := testConfig()
	cfg.IPLimit = 1
	g := newGuard(checker, newMemoryBox(), cfg)
	r := testRouter(g)

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "2.2.2.2:1"
		req.Header.Set("X-Real-IP", "2.2.2.2")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("ping status=%d", w.Code)
		}
	}
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/wallet/stripe/notify", nil)
		req.RemoteAddr = "2.2.2.2:1"
		req.Header.Set("X-Real-IP", "2.2.2.2")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("notify status=%d", w.Code)
		}
	}
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/static/videos/a.mp4", nil)
		req.RemoteAddr = "2.2.2.2:1"
		req.Header.Set("X-Real-IP", "2.2.2.2")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("static status=%d", w.Code)
		}
	}
}

func TestPenaltyTightensLimit(t *testing.T) {
	checker := &stubChecker{}
	box := newMemoryBox()
	g := newGuard(checker, box, testConfig())
	r := testRouter(g)

	// 前 3 次通过，第 4 次拒绝并记 1 次 deny；第 5 次再拒绝，deny 达到阈值，进入处罚箱。
	for i := 0; i < 5; i++ {
		doPOST(r, "/feed/listRecommend", "3.3.3.3")
	}
	if _, ok := box.penalties["ip:3.3.3.3"]; !ok {
		t.Fatalf("expected penalty, denies=%v penalties=%v", box.denies, box.penalties)
	}

	// 处罚后限额变成 1。计数器不重置，下一次必然拒绝。
	w := doPOST(r, "/feed/listRecommend", "3.3.3.3")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("penalized request status=%d", w.Code)
	}
}

func TestFailOpenOnCheckerError(t *testing.T) {
	checker := &stubChecker{err: context.DeadlineExceeded}
	g := newGuard(checker, newMemoryBox(), testConfig())
	r := testRouter(g)
	w := doPOST(r, "/feed/listRecommend", "4.4.4.4")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestInFlightShedding(t *testing.T) {
	cfg := testConfig()
	cfg.MaxInFlight = 1
	cfg.IPLimit = 100
	g := newGuard(&stubChecker{}, newMemoryBox(), cfg)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(g.Middleware())
	started := make(chan struct{})
	release := make(chan struct{})
	r.GET("/hold", func(c *gin.Context) {
		close(started)
		<-release
		c.Status(http.StatusOK)
	})
	r.GET("/other", func(c *gin.Context) { c.Status(http.StatusOK) })

	done := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/hold", nil)
		req.RemoteAddr = "5.5.5.5:1"
		r.ServeHTTP(w, req)
		done <- w.Code
	}()
	<-started

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/other", nil)
	req.RemoteAddr = "5.5.5.5:1"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("shed status=%d", w.Code)
	}
	close(release)
	if code := <-done; code != http.StatusOK {
		t.Fatalf("held request status=%d", code)
	}
}

func TestAccountCeiling(t *testing.T) {
	cfg := testConfig()
	cfg.IPLimit = 100
	cfg.AccountLimit = 2
	g := newGuard(&stubChecker{}, newMemoryBox(), cfg)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("account_id", uint64(9))
		c.Next()
	})
	r.Use(g.Middleware())
	r.POST("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	var last *httptest.ResponseRecorder
	for i := 0; i < 3; i++ {
		last = httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = "6.6.6.6:1"
		req.Header.Set("X-Real-IP", "6.6.6.6")
		r.ServeHTTP(last, req)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d", last.Code)
	}
}
