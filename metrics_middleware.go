package main

import (
	"net/http"
	"strconv"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		// don't track metrics endpoint
		if r.URL.Path == "/metrics" {
			return
		}

		path := r.URL.Path
		method := r.Method
		status := strconv.Itoa(recorder.status)

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
	})
}
