package vii

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrRateLimited = errors.New("vii: rate limited")

type RateLimitMetrics interface {
	Allowed(ip string)
	Limited(ip string)
	Skipped(ip string, reason string)
	Evicted(ip string)
}

type rateLimitNoopMetrics struct{}

func (rateLimitNoopMetrics) Allowed(_ string)           {}
func (rateLimitNoopMetrics) Limited(_ string)           {}
func (rateLimitNoopMetrics) Skipped(_ string, _ string) {}
func (rateLimitNoopMetrics) Evicted(_ string)           {}

type RateLimitService struct {
	MaxEntries int
	Burst      int
	RefillEvery time.Duration

	Key  func(r *http.Request) string
	Skip func(r *http.Request) (bool, string)

	SetRetryAfterHeader bool
	Now                func() time.Time
	Metrics            RateLimitMetrics

	mu    sync.Mutex
	state map[string]*ipState
}

type ipState struct {
	tokens   float64
	last     time.Time
	lastSeen time.Time
}

func (s *RateLimitService) withDefaults() *RateLimitService {
	if s == nil {
		return s
	}
	if s.MaxEntries <= 0 {
		s.MaxEntries = 10_000
	}
	if s.Burst <= 0 {
		s.Burst = 20
	}
	if s.RefillEvery <= 0 {
		s.RefillEvery = 50 * time.Millisecond // ~20 req/s
	}
	if s.Key == nil {
		s.Key = defaultRateLimitKey
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if !s.SetRetryAfterHeader {
		s.SetRetryAfterHeader = true
	}
	if s.Metrics == nil {
		s.Metrics = rateLimitNoopMetrics{}
	}
	if s.state == nil {
		s.state = make(map[string]*ipState, 1024)
	}
	return s
}

func defaultRateLimitKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		return xr
	}
	host := r.RemoteAddr
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil && h != "" {
		return h
	}
	return host
}

func (s *RateLimitService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	if r == nil || w == nil {
		return r, nil
	}
	s = s.withDefaults()
	if s == nil {
		return r, nil
	}

	if s.Skip != nil {
		if ok, reason := s.Skip(r); ok {
			ip := s.Key(r)
			s.Metrics.Skipped(ip, reason)
			return r, nil
		}
	}

	ip := s.Key(r)
	if ip == "" {
		s.Metrics.Skipped("", "missing_key")
		return r, nil
	}

	now := s.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.state[ip]
	if st == nil {
		if len(s.state) >= s.MaxEntries {
			evicted := evictOldest(s.state)
			if evicted != "" {
				s.Metrics.Evicted(evicted)
			}
		}
		st = &ipState{
			tokens:   float64(s.Burst),
			last:     now,
			lastSeen: now,
		}
		s.state[ip] = st
	}

	if now.After(st.last) && s.RefillEvery > 0 {
		elapsed := now.Sub(st.last)
		add := float64(elapsed) / float64(s.RefillEvery)
		if add > 0 {
			st.tokens += add
			if st.tokens > float64(s.Burst) {
				st.tokens = float64(s.Burst)
			}
			steps := int64(elapsed / s.RefillEvery)
			if steps > 0 {
				st.last = st.last.Add(time.Duration(steps) * s.RefillEvery)
			} else {
				st.last = now
			}
		}
	}

	st.lastSeen = now

	if st.tokens >= 1.0 {
		st.tokens -= 1.0
		s.Metrics.Allowed(ip)
		return r, nil
	}

	s.Metrics.Limited(ip)

	if s.SetRetryAfterHeader {
		retry := s.RefillEvery
		if retry <= 0 {
			retry = time.Second
		}
		secs := int64(retry.Seconds())
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.FormatInt(secs, 10))
	}

	return r, ErrRateLimited
}

func (s *RateLimitService) After(r *http.Request, w http.ResponseWriter) error {
	_ = r
	_ = w
	return nil
}

func evictOldest(m map[string]*ipState) string {
	var (
		oldestKey  string
		oldestTime time.Time
		init       bool
	)
	for k, v := range m {
		if v == nil {
			continue
		}
		if !init || v.lastSeen.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.lastSeen
			init = true
		}
	}
	if oldestKey != "" {
		delete(m, oldestKey)
	}
	return oldestKey
}

func RateLimitDefault() RateLimitService { return RateLimitService{} }

func RateLimitRPS(rps int, burst int) RateLimitService {
	if rps <= 0 {
		rps = 20
	}
	if burst <= 0 {
		burst = 20
	}
	return RateLimitService{
		Burst:       burst,
		RefillEvery: time.Second / time.Duration(rps),
	}
}
