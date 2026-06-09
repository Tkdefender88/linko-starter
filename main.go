package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
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

func initializeLogger(logFile string) (*log.Logger, func() error, error) {
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
		return log.New(mWriter, logPrefix, log.LstdFlags), closer, nil
	}

	noOpCloser := func() error { return nil }
	return log.New(os.Stderr, logPrefix, log.LstdFlags), noOpCloser, nil
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
		logger.Printf("failed to create store: %v\n", err)
		return 1
	}
	s := newServer(*st, httpPort, cancel, logger)
	var serverErr error
	go func() {
		logger.Printf("Linko is running on http://localhost:%d", httpPort)
		serverErr = s.start()
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger.Printf("Linko is shutting down")
	if err := s.shutdown(shutdownCtx); err != nil {
		logger.Printf("failed to shutdown server: %v\n", err)
		return 1
	}
	if serverErr != nil {
		logger.Printf("server error: %v\n", serverErr)
		return 1
	}
	return 0
}
