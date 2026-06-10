package main

import (
	"io"
	"log/slog"
	"net/http"
	"time"
)

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

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			spyWriter := &spyResponseWriter{ResponseWriter: w}
			spyRequest := &spyReadCloser{ReadCloser: r.Body}
			r.Body = spyRequest

			start := time.Now()
			next.ServeHTTP(spyWriter, r)
			duration := time.Since(start)

			logger.Info("Served request",
				slog.Int("request_body_bytes", spyRequest.bytesRead),
				slog.Int("response_body_bytes", spyWriter.bytesWritten),
				slog.Int("response_status", spyWriter.statusCode),
				slog.Duration("duration", duration),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("client_ip", r.RemoteAddr),
			)
		})
	}
}
