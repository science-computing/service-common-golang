package auditlog

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/apex/log"
)

var Log log.Interface = &log.Logger{
	Handler: New(os.Stdout),
	Level:   log.InfoLevel,
}

type Handler struct {
	mutex  sync.Mutex
	Writer io.Writer
}

func New(w io.Writer) *Handler {
	return &Handler{
		Writer: w,
	}
}

func (h *Handler) HandleLog(e *log.Entry) error {
	color := 32 // color green
	names := e.Fields.Names()

	h.mutex.Lock()
	defer h.mutex.Unlock()

	ts := time.Now()

	fmt.Fprintf(h.Writer, "\033[%dm%6s\033[0m[%s] %-25s", color, "AUDIT", ts.Format("2006-02-01 15:04:05"), e.Message)

	for _, name := range names {
		fmt.Fprintf(h.Writer, " \033[%dm%s\033[0m=%v", color, name, e.Fields.Get(name))
	}

	fmt.Fprintln(h.Writer)

	return nil
}

func SetLevel(l log.Level) {
	if logger, ok := Log.(*log.Logger); ok {
		logger.Level = l
	}
}

func WithFields(fields log.Fielder) *log.Entry {
	return Log.WithFields(fields)
}
