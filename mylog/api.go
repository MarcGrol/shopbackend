package mylog

import "context"

type Severity string

const (
	SeverityDebug Severity = "DEBUG"
	SeverityInfo  Severity = "INFO"
	SeverityWarn  Severity = "WARN"
	SeverityError Severity = "ERROR"
)

var New func(name string) Logger

type Logger interface {
	Log(ctx context.Context, traceLabel string, severity Severity, format string, a ...any)
}
