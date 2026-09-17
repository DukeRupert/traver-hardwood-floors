package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// healthPath is polled by uptime checks; its requests are logged at DEBUG so
// they do not flood the shared Loki instance.
const healthPath = "/health"

// trustedProxyHops is the number of reverse proxies in front of this service
// that append their own entry to X-Forwarded-For. The real client IP is the
// entry our own proxy wrote — the right-most one, minus these hops.
//
// Today the chain is: visitor -> host Caddy (trusted_proxies unset, so it
// REPLACES X-Forwarded-For with the real peer) -> in-container Caddy
// (trusted_proxies private_ranges, so it APPENDS its peer, the host Caddy) ->
// this service. What reaches us is therefore:
//
//	X-Forwarded-For: <real client>, <docker gateway>
//
// i.e. one hop to skip from the right. If a CDN (e.g. Cloudflare) is ever put
// in front and the host Caddy gains trusted_proxies, the chain grows by one
// entry and this constant must be bumped to match.
const trustedProxyHops = 1

// stdlogBridge routes anything still writing through the standard library's
// log package (net/http's own error output, third-party packages) into the
// structured logger, so the process never emits unstructured text.
type stdlogBridge struct{}

func (stdlogBridge) Write(p []byte) (int, error) {
	slog.Error("stdlib log", "line", strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// setupLogger installs a JSON slog handler on stdout as the default logger and
// returns a *log.Logger suitable for http.Server.ErrorLog. slog's JSON handler
// already emits `time` (RFC3339 with nanoseconds), uppercase `level` and `msg`.
func setupLogger() *log.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(os.Getenv("LOG_LEVEL"), "debug") {
		level = slog.LevelDebug
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})))

	log.SetFlags(0)
	log.SetOutput(stdlogBridge{})

	return log.New(stdlogBridge{}, "", 0)
}

type ctxKey int

const reqStateKey ctxKey = 0

// reqState carries the per-request id and lets a handler mark the request as
// failed so the access log line becomes "request failed" at ERROR.
type reqState struct {
	id  string
	err error
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Time-based fallback: uniqueness matters, secrecy does not.
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// requestLogger returns a logger carrying the current request_id, so every
// line emitted while serving a request can be correlated with its access log.
func requestLogger(r *http.Request) *slog.Logger {
	if st, ok := r.Context().Value(reqStateKey).(*reqState); ok {
		return slog.Default().With("request_id", st.id)
	}
	return slog.Default()
}

// failRequest records err against the request so it is logged as
// "request failed" with an `error` field.
func failRequest(r *http.Request, err error) {
	if st, ok := r.Context().Value(reqStateKey).(*reqState); ok {
		st.err = err
	}
}

// statusRecorder captures the status code written by the handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// logRequests emits exactly one line per HTTP request.
func logRequests(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		st := &reqState{id: newRequestID()}
		r = r.WithContext(context.WithValue(r.Context(), reqStateKey, st))

		rec := &statusRecorder{ResponseWriter: w}
		rec.Header().Set("X-Request-Id", st.id)

		next(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}

		attrs := []any{
			"request_id", st.id,
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", status,
			"duration_ms", float64(time.Since(start)) / float64(time.Millisecond),
			"remote_ip", clientIP(r),
			"user_agent", r.UserAgent(),
			"referer", r.Referer(),
		}

		switch {
		case r.URL.Path == healthPath:
			slog.Debug("request", attrs...)
		case st.err != nil:
			slog.Error("request failed", append(attrs, "error", st.err.Error())...)
		case status >= 500:
			slog.Error("request failed", attrs...)
		default:
			// 4xx is normal traffic and stays at INFO.
			slog.Info("request", attrs...)
		}
	}
}

// clientIP returns the real visitor IP, without a port. See trustedProxyHops
// for why the chain is walked from the right.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		i := len(parts) - 1 - trustedProxyHops
		if i < 0 {
			i = 0
		}
		if ip := strings.TrimSpace(parts[i]); ip != "" {
			return stripPort(ip)
		}
	}
	return stripPort(r.RemoteAddr)
}

func stripPort(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
