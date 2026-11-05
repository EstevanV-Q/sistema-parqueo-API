package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

// New crea un logger estructurado que escribe en stdout en formato JSON.
func New(level string, out io.Writer) *Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     lvl,
		AddSource: true,
	}
	handler := slog.NewJSONHandler(out, opts)
	return &Logger{slog.New(handler)}
}

// Default devuelve un logger que escribe a stdout con nivel info.
func Default() *Logger {
	return New("info", os.Stdout)
}

// WithRequestID enriquece el logger con request_id y user_id cuando están presentes.
func (l *Logger) WithRequestID(ctx context.Context) *Logger {
	attrs := []slog.Attr{}
	if rid, ok := ctx.Value("request_id").(string); ok && rid != "" {
		attrs = append(attrs, slog.String("request_id", rid))
	}
	if uid, ok := ctx.Value("user_id").(string); ok && uid != "" {
		attrs = append(attrs, slog.String("user_id", uid))
	}
	if len(attrs) > 0 {
		anyAttrs := make([]any, len(attrs))
		for i, a := range attrs {
			anyAttrs[i] = a
		}
		return &Logger{l.With(anyAttrs...)}
	}
	return l
}
