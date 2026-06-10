package main

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"boot.dev/linko/internal/linkoerr"
	pkgerr "github.com/pkg/errors"
)

type StackTracer interface {
	error
	StackTrace() pkgerr.StackTrace
}

type multiError interface {
	error
	Unwrap() []error
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "error" {
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}
		return errorAttrs(err)
	}
	return a
}

func errorAttrs(err error) slog.Attr {
	errAttrs := []slog.Attr{}
	if me, ok := errors.AsType[multiError](err); ok {
		var attrs []slog.Attr
		for i, e := range me.Unwrap() {
			attrs = append(attrs, slog.String(fmt.Sprintf("error_%d", i+1), e.Error()))
		}
		return slog.GroupAttrs("errors", attrs...)
	}

	errAttrs = append(errAttrs, slog.String("message", err.Error()))
	errAttrs = append(errAttrs, linkoerr.Attrs(err)...)

	if stackErr, ok := errors.AsType[StackTracer](err); ok {
		errAttrs = append(errAttrs,
			slog.String("stack_trace", fmt.Sprintf("%+v", stackErr.StackTrace())))
	}

	return slog.GroupAttrs("error", errAttrs...)
}

func initializeLogger(logFile string) (*slog.Logger, func() error, error) {
	const logPrefix = ""

	if logFile != "" {
		debugHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:       slog.LevelDebug,
			ReplaceAttr: replaceAttr,
		})

		fd, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("error opening the log file: %w", err)
		}

		bufferedFile := bufio.NewWriterSize(fd, 8192)

		infoHandler := slog.NewJSONHandler(bufferedFile, &slog.HandlerOptions{
			Level:       slog.LevelInfo,
			ReplaceAttr: replaceAttr,
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
