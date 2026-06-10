package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"boot.dev/linko/internal/linkoerr"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	pkgerr "github.com/pkg/errors"
	"gopkg.in/natefinch/lumberjack.v2"
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
		return replaceErrAttr(err)
	}
	return a
}

func replaceErrAttr(err error) slog.Attr {
	errAttrs := []slog.Attr{}
	if me, ok := errors.AsType[multiError](err); ok {
		var attrs []slog.Attr
		for i, e := range me.Unwrap() {
			attrs = append(attrs, slog.Any(fmt.Sprintf("error_%d", i+1), linkoerr.Attrs(e)))
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

func isTerminal() bool {
	return isatty.IsCygwinTerminal(os.Stdout.Fd()) || isatty.IsTerminal(os.Stdout.Fd())
}

func initializeLogger(logFile string) (*slog.Logger, func() error, error) {

	handlers := []slog.Handler{
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:       slog.LevelDebug,
			ReplaceAttr: replaceAttr,
			NoColor:     !isTerminal(),
		}),
	}

	closers := []func() error{}

	if logFile != "" {
		lumberLogger := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    1,
			MaxAge:     28,
			MaxBackups: 10,
			LocalTime:  false,
			Compress:   true,
		}
		infoHandler := slog.NewJSONHandler(lumberLogger, &slog.HandlerOptions{
			Level:       slog.LevelInfo,
			ReplaceAttr: replaceAttr,
		})

		handlers = append(handlers, infoHandler)
		closers = append(closers, func() error {
			if err := lumberLogger.Close(); err != nil {
				return fmt.Errorf("failed to close lumberjack logger: %w", err)
			}
			return nil
		})
	}

	closeFunc := func() error {
		errs := []error{}
		for _, closer := range closers {
			err := closer()
			if err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	return slog.New(slog.NewMultiHandler(handlers...)), closeFunc, nil
}
