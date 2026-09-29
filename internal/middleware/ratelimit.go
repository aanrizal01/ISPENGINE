package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"isp-onboarding/pkg/response"
)

// clientRecord menyimpan histori timestamp request untuk tiap client IP
type clientRecord struct {
	timestamps []time.Time
	lastSeen   time.Time
}

// IPRateLimiter mengontrol frekuensi pemanggilan endpoint per alamat IP
type IPRateLimiter struct {
	mu          sync.Mutex
	clients     map[string]*clientRecord
	maxRequests int
	window      time.Duration
}

// NewIPRateLimiter membuat rate limiter baru dengan kuota maxRequests dalam durasi window
func NewIPRateLimiter(maxRequests int, window time.Duration) *IPRateLimiter {
	limiter := &IPRateLimiter{
		clients:     make(map[string]*clientRecord),
		maxRequests: maxRequests,
		window:      window,
	}

	// Goroutine pembersihan record usang secara berkala (setiap 10 menit)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			limiter.mu.Lock()
			cutoff := time.Now().Add(-limiter.window * 2)
			for ip, record := range limiter.clients {
				if record.lastSeen.Before(cutoff) {
					delete(limiter.clients, ip)
				}
			}
			limiter.mu.Unlock()
		}
	}()

	return limiter
}

// getIP mengekstrak IP klien dengan memperhatikan header reverse proxy (X-Forwarded-For, X-Real-IP)
func getIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// Limit membungkus http.Handler dengan verifikasi batas rate request
func (l *IPRateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)

		l.mu.Lock()
		now := time.Now()
		record, exists := l.clients[ip]
		if !exists {
			record = &clientRecord{
				timestamps: []time.Time{now},
				lastSeen:   now,
			}
			l.clients[ip] = record
			l.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		record.lastSeen = now
		// Bersihkan timestamp yang sudah berada di luar window waktu
		cutoff := now.Add(-l.window)
		valid := make([]time.Time, 0, len(record.timestamps))
		for _, ts := range record.timestamps {
			if ts.After(cutoff) {
				valid = append(valid, ts)
			}
		}

		if len(valid) >= l.maxRequests {
			l.mu.Unlock()
			response.Error(w, http.StatusTooManyRequests, "Terlalu banyak permintaan (Rate Limit Exceeded). Silakan coba lagi beberapa saat lagi.", "rate_limited")
			return
		}

		record.timestamps = append(valid, now)
		l.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}
