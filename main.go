package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boot.dev/linko/internal/store"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	httpPort := flag.Int("port", 8899, "port to listen on")
	dataDir := flag.String("data", "./data", "directory to store data")
	flag.Parse()

	status := run(ctx, cancel, *httpPort, *dataDir)
	cancel()
	os.Exit(status)
}

func initializeLogger(logFile string) (*slog.Logger, func() error, error) {
	const logPrefix = ""

	if logFile != "" {
		fd, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("error opening the log file: %w", err)
		}

		bufferedFile := bufio.NewWriterSize(fd, 8192)

		closer := func() error {
			if err := bufferedFile.Flush(); err != nil {
				return fmt.Errorf("error flushing the log buffer: %w", err)
			}
			if err := fd.Close(); err != nil {
				return fmt.Errorf("error closing the log file: %w", err)
			}
			return nil
		}

		mWriter := io.MultiWriter(bufferedFile, os.Stderr)
		return slog.New(slog.NewTextHandler(mWriter, nil)), closer, nil
	}

	noOpCloser := func() error { return nil }
	return slog.New(slog.NewTextHandler(os.Stderr, nil)), noOpCloser, nil
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort int, dataDir string) int {

	logger, closer, err := initializeLogger(os.Getenv("LINKO_LOG_FILE"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v", err)
		return 1
	}
	defer func() {
		if err := closer(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close logger: %v", err)
		}
	}()

	st, err := store.New(dataDir, logger)
	if err != nil {
		logger.Info(fmt.Sprintf("failed to create store: %v\n", err))
		return 1
	}
	s := newServer(*st, httpPort, cancel, logger)
	var serverErr error
	go func() {
		logger.Info(fmt.Sprintf("Linko is running on http://localhost:%d", httpPort))
		serverErr = s.start()
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger.Info("Linko is shutting down")
	if err := s.shutdown(shutdownCtx); err != nil {
		logger.Info("failed to shutdown server", "error", err)
		return 1
	}
	if serverErr != nil {
		logger.Info("server error", "error", serverErr)
		return 1
	}
	return 0
}
