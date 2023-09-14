package verbosetextlog

import (
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	apexlog "github.com/apex/log"
	"github.com/apex/log/handlers/text"
)

type Handler struct {
	mutex  sync.Mutex
	Writer io.Writer
}

func New(w io.Writer) *Handler {
	return &Handler{
		Writer: w,
	}
}

func (h *Handler) HandleLog(e *apexlog.Entry) error {
	color := text.Colors[e.Level]
	level := text.Strings[e.Level]
	names := e.Fields.Names()

	h.mutex.Lock()
	defer h.mutex.Unlock()

	ts := time.Now()

	var file string
	var line int
	var ok bool
	// try to guess the number of stack frames to skip
	// This might break with major internal changes to the library
	for i := 4; i < 10; i++ {
		_, file, line, ok = runtime.Caller(i)
		if !ok {
			break
		}
		if !strings.Contains(file, "github.com/apex/log") {
			break
		}
	}

	if file != "" {
		file = filepath.Base(file)
	}

	fmt.Fprintf(h.Writer, "\033[%dm%6s\033[0m[%s] %-25s -- %s:%d", color, level, ts.Format("2006-01-02 15:04:05"), e.Message, file, line)

	for _, name := range names {
		fmt.Fprintf(h.Writer, " \033[%dm%s\033[0m=%v", color, name, e.Fields.Get(name))
	}

	fmt.Fprintln(h.Writer)

	return nil
}
