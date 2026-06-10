package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const LogContextKey contextKey = "log_context"

type LogContext struct {
	Username string
	Error    error
}

type spyReadCloser struct {
	io.ReadCloser
	bytesRead int
}

func (s *spyReadCloser) Read(p []byte) (int, error) {
	n, err := s.ReadCloser.Read(p)
	s.bytesRead += n
	return n, err
}

type spyResponseWriter struct {
	http.ResponseWriter
	bytesWritten int
	statusCode   int
}

func (s *spyResponseWriter) Write(p []byte) (int, error) {
	if s.statusCode == 0 {
		s.statusCode = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(p)
	s.bytesWritten += n
	return n, err
}

func (s *spyResponseWriter) WriteHeader(statusCode int) {
	s.statusCode = statusCode
	s.ResponseWriter.WriteHeader(statusCode)
}

func httpError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	if logCtx, ok := ctx.Value(LogContextKey).(*LogContext); ok {
		logCtx.Error = err
	}
	http.Error(w, err.Error(), status)
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Add logging context pointer to the request context
			lc := new(LogContext)
			ctx := context.WithValue(r.Context(), LogContextKey, lc)
			r = r.WithContext(ctx)

			// use the spy request and response writer to capture bytes read/written and the status code
			spyWriter := &spyResponseWriter{ResponseWriter: w}
			spyRequest := &spyReadCloser{ReadCloser: r.Body}
			r.Body = spyRequest

			start := time.Now()
			next.ServeHTTP(spyWriter, r)
			duration := time.Since(start)

			logAttrs := []slog.Attr{
				slog.Int("request_body_bytes", spyRequest.bytesRead),
				slog.Int("response_body_bytes", spyWriter.bytesWritten),
				slog.Int("response_status", spyWriter.statusCode),
				slog.Duration("duration", duration),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("request_id", r.Header.Get("X-Request-ID")),
				slog.String("client_ip", r.RemoteAddr)}
			if logContext, ok := r.Context().Value(LogContextKey).(*LogContext); ok {
				if logContext.Username != "" {
					logAttrs = append(logAttrs, slog.String("user", logContext.Username))
				}
				if logContext.Error != nil {
					logAttrs = append(logAttrs, slog.Any("error", logContext.Error))
				}
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "Served request", logAttrs...)
		})
	}
}
