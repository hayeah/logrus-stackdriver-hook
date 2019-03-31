package hook

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"

	"cloud.google.com/go/errorreporting"
	"cloud.google.com/go/logging"

	"github.com/sirupsen/logrus"
)

// DefaultErrorLevels are default log levels suitable for error reporting
var DefaultErrorLevels = []logrus.Level{
	logrus.PanicLevel,
	logrus.FatalLevel,
	logrus.ErrorLevel,
}

// DefaultLogLevels are default log levels suitable for sending to log collector
var DefaultLogLevels = []logrus.Level{
	logrus.WarnLevel,
	logrus.InfoLevel,
}

// ErrorReport sends log events to stackdriver error report service
type ErrorReport struct {
	client *errorreporting.Client
	levels []logrus.Level
}

// NewErrorReport instantiates a ErrorReport
func NewErrorReport(client *errorreporting.Client, levels ...logrus.Level) *ErrorReport {
	if levels == nil {
		levels = DefaultErrorLevels
	}

	return &ErrorReport{
		client: client,
		levels: levels,
	}
}

// Fire implements logrus.Hook
func (h *ErrorReport) Fire(e *logrus.Entry) error {
	var buf [16 * 1024]byte

	n := runtime.Stack(buf[:], false)
	callerStack := chopstack(buf[:n])

	var user string
	if val, ok := e.Data["user"]; ok {
		switch val := val.(type) {
		case string:
			user = val
		default:
			user = fmt.Sprintf("%v", val)
		}
	}

	h.client.Report(errorreporting.Entry{
		Error: errors.New(e.Message),
		Stack: callerStack,
		// User  string        // an identifier for the user affected by the error
		User: user,
		// Req   *http.Request // if error is associated with a request.
	})

	return nil
}

// Levels implements logrus.Hook
func (h *ErrorReport) Levels() []logrus.Level {
	return h.levels
}

// Log sends events to stackdriver logging service
type Log struct {
	logger *logging.Logger
	levels []logrus.Level
}

// NewLog returns a LogHook
func NewLog(logger *logging.Logger, levels ...logrus.Level) *Log {
	if levels == nil {
		levels = DefaultLogLevels
	}

	return &Log{
		logger: logger,
		levels: levels,
	}
}

// Fire implements logrus.Hook
func (h *Log) Fire(e *logrus.Entry) error {
	h.logger.Log(h.toEntry(e))

	return nil
}

// Levels implements logrus.Hook
func (h *Log) Levels() []logrus.Level {
	return h.levels
}

func (h *Log) toEntry(e *logrus.Entry) logging.Entry {

	labels := make(map[string]string, len(e.Data))

	// var httpReq *logging.HTTPRequest

	for k, v := range e.Data {
		switch v := v.(type) {
		case string:
			labels[k] = v
		// case *http.Request:
		// 	httpReq = &logging.HTTPRequest{
		// 		Referer:       v.Referer(),
		// 		RemoteIp:      v.RemoteAddr,
		// 		RequestMethod: v.Method,
		// 		RequestUrl:    v.URL.String(),
		// 		UserAgent:     v.UserAgent(),
		// 	}

		// case *logging.HttpRequest:
		// 	httpReq = x
		default:
			labels[k] = fmt.Sprintf("%v", v)
		}
	}

	// TODO: support "caller"
	// TODO: support "stack"
	// TODO: support severity field to override default mapping from level

	return logging.Entry{
		Timestamp: e.Time,
		Severity:  levelToSeverity(e.Level),
		Payload:   e.Message,
		Labels:    labels,
	}

}

func levelToSeverity(l logrus.Level) logging.Severity {
	switch l {
	case logrus.InfoLevel:
		return logging.Info
	case logrus.WarnLevel:
		return logging.Warning
	case logrus.ErrorLevel:
		return logging.Error
	case logrus.PanicLevel, logrus.FatalLevel:
		return logging.Critical
	default:
		return logging.Default
	}
}

func chopstack(buf []byte) []byte {
	// stack trace looks something like the following. We wkip over all log internal lines
	// line goroutine 1 [running]:
	// line github.com/hayeah/logrus-stackdriver-hook.(*ErrorReport).Fire(0xc0000bcae0, 0xc0001a8a10, 0x2, 0xc0001d6148)
	// line    /Users/howard/src/logrus-stackdriver-hook/hook.go:41 +0x6c
	// line github.com/sirupsen/logrus.LevelHooks.Fire(0xc0000a21b0, 0x2, 0xc0001a8a10, 0x863010d3dc30c, 0xc02278d058)
	// line    /Users/howard/go/pkg/mod/github.com/sirupsen/logrus@v1.4.0/hooks.go:28 +0x91
	// line github.com/sirupsen/logrus.(*Entry).fireHooks(0xc0001a8a10)
	// line    /Users/howard/go/pkg/mod/github.com/sirupsen/logrus@v1.4.0/entry.go:247 +0x8c
	// line github.com/sirupsen/logrus.Entry.log(0xc0000b8120, 0xc0001c4b40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, ...)
	// line    /Users/howard/go/pkg/mod/github.com/sirupsen/logrus@v1.4.0/entry.go:225 +0xf6
	// line github.com/sirupsen/logrus.(*Entry).Log(0xc0001a89a0, 0xc000000002, 0xc00025bf08, 0x1, 0x1)
	// line    /Users/howard/go/pkg/mod/github.com/sirupsen/logrus@v1.4.0/entry.go:269 +0xc8
	// line github.com/sirupsen/logrus.(*Logger).Log(0xc0000b8120, 0x2, 0xc00025bf08, 0x1, 0x1)
	// line    /Users/howard/go/pkg/mod/github.com/sirupsen/logrus@v1.4.0/logger.go:192 +0x7d
	// line github.com/sirupsen/logrus.(*Logger).Error(0xc0000b8120, 0xc00025bf08, 0x1, 0x1)
	// line    /Users/howard/go/pkg/mod/github.com/sirupsen/logrus@v1.4.0/logger.go:224 +0x51
	// line github.com/sirupsen/logrus.Error(0xc00025bf08, 0x1, 0x1)
	// line    /Users/howard/go/pkg/mod/github.com/sirupsen/logrus@v1.4.0/exported.go:124 +0x4b
	// line main.dologs(0x0, 0x0)
	// line    /Users/howard/src/logrus-stackdriver-hook/example/main.go:33 +0x2d3
	// line main.main()
	// line    /Users/howard/src/logrus-stackdriver-hook/example/main.go:39 +0x22

	lines := bytes.Split(buf, []byte{'\n'})

	i := 3
	for {
		line := lines[i]
		if !bytes.HasPrefix(line, []byte("github.com/sirupsen/logrus.")) {
			break
		}

		i += 2
	}

	// We can't omit the first line, or else the RPC would reject the error entry
	// because it can't recognize the stackframe.
	//
	// See:
	// https://github.com/googleapis/google-cloud-go/issues/1084
	return bytes.Join(append([][]byte{lines[0]}, lines[i:]...), []byte("\n"))
}
