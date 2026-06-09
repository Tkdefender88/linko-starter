package main

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"

	pkgerr "github.com/pkg/errors"
)

type StackTracer interface {
	error
	StackTrace() pkgerr.StackTrace
}

func stackTraceErrors(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "error" {
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}

		if stackErr, ok := errors.AsType[StackTracer](err); ok {
			return slog.GroupAttrs(a.Key, slog.Attr{
				Key:   "message",
				Value: slog.StringValue(stackErr.Error()),
			}, slog.Attr{
				Key:   "stack_trace",
				Value: slog.StringValue(fmt.Sprintf("%+v", stackErr.StackTrace())),
			})
		}
		return slog.String(a.Key, fmt.Sprintf("%+v", err))
	}
	return a
}

func initializeLogger(logFile string) (*slog.Logger, func() error, error) {
	const logPrefix = ""

	if logFile != "" {
		debugHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:       slog.LevelDebug,
			ReplaceAttr: stackTraceErrors,
		})

		fd, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("error opening the log file: %w", err)
		}

		bufferedFile := bufio.NewWriterSize(fd, 8192)

		infoHandler := slog.NewJSONHandler(bufferedFile, &slog.HandlerOptions{
			Level:       slog.LevelInfo,
			ReplaceAttr: stackTraceErrors,
		})

		closer := func() error {
			if err := bufferedFile.Flush(); err != nil {
				return fmt.Errorf("error flushing the log buffer: %w", err)
			}
			if err := fd.Close(); err != nil {
				return fmt.Errorf("error closing the log file: %w", err)
			}
			return nil
		}

		mHandler := slog.NewMultiHandler(
			debugHandler,
			infoHandler,
		)

		return slog.New(mHandler), closer, nil
	}

	noOpCloser := func() error { return nil }
	return slog.New(slog.NewTextHandler(os.Stderr, nil)), noOpCloser, nil
}
