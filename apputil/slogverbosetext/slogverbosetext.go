package slogverbosetext

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync"
) // Color codes for different log levels (matching apex/log text handler)
var Colors = map[slog.Level]int{
	slog.LevelDebug: 37, // white
	slog.LevelInfo:  36, // cyan
	slog.LevelWarn:  33, // yellow
	slog.LevelError: 31, // red
}

// Level strings (matching apex/log text handler)
var Strings = map[slog.Level]string{
	slog.LevelDebug: "DEBUG",
	slog.LevelInfo:  "INFO",
	slog.LevelWarn:  "WARN",
	slog.LevelError: "ERROR",
}

type Handler struct {
	mutex sync.Mutex
	w     io.Writer
	opts  slog.HandlerOptions
}

func New(w io.Writer, opts *slog.HandlerOptions) *Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &Handler{
		w:    w,
		opts: *opts,
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Get color and level string
	color, exists := Colors[r.Level]
	if !exists {
		color = 37 // default to white
	}

	levelStr, exists := Strings[r.Level]
	if !exists {
		levelStr = r.Level.String()
	}

	// Format timestamp
	ts := r.Time.Format("2006-01-02 15:04:05")

	// Get caller information
	var file string
	var line int
	if h.opts.AddSource && r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			file = filepath.Base(f.File)
			line = f.Line
		}
	}

	// Format: [COLOR]LEVEL[/COLOR][TIMESTAMP] MESSAGE -- FILE:LINE
	fmt.Fprintf(h.w, "\033[%dm%6s\033[0m[%s] %-25s", color, levelStr, ts, r.Message)

	if file != "" {
		fmt.Fprintf(h.w, " -- %s:%d", file, line)
	}

	// Add structured attributes with colors
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.w, " \033[%dm%s\033[0m=%v", color, a.Key, a.Value)
		return true
	})

	fmt.Fprintln(h.w)
	return nil
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For simplicity, we'll create a new handler with the same options
	// In a full implementation, you might want to store and use these attrs
	return &Handler{
		w:    h.w,
		opts: h.opts,
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	// For simplicity, we'll create a new handler with the same options
	// In a full implementation, you might want to handle groups
	return &Handler{
		w:    h.w,
		opts: h.opts,
	}
}
