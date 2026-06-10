package linkoerr

import (
	"errors"
	"log/slog"
)

type errorWithAttr struct {
	error
	attrs []slog.Attr
}

type attrError interface {
	Attrs() []slog.Attr
}

func WithAttrs(err error, args ...any) error {
	return &errorWithAttr{
		error: err,
		attrs: argsToAttrs(args...),
	}
}

func (e *errorWithAttr) Unwrap() error {
	return e.error
}

func (e *errorWithAttr) Attrs() []slog.Attr {
	return e.attrs
}

func Attrs(err error) []slog.Attr {
	var attrs []slog.Attr
	for err != nil {
		if ae, ok := err.(attrError); ok {
			attrs = append(attrs, ae.Attrs()...)
		}
		err = errors.Unwrap(err)
	}
	return attrs
}

func argsToAttrs(args ...any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(args)/2)
	for i := 0; i < len(args); {
		switch key := args[i].(type) {
		case string:
			if i+1 >= len(args) {
				attrs = append(attrs, slog.String("!BADKEY", key))
				i++
			} else {
				attrs = append(attrs, slog.Any(key, args[i+1]))
				i += 2
			}
		case slog.Attr:
			attrs = append(attrs, key)
			i += 1
		default:
			attrs = append(attrs, slog.Any("!BADKEY", args[i]))
			i += 1
			continue
		}
	}
	return attrs
}
