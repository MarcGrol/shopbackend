package mylog

import (
	"context"
	"fmt"
	"os"
)

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		New = newStandardLogger
	}
}

type standardLogger struct {
	componentName string
}

func newStandardLogger(componentName string) Logger {
	return standardLogger{
		componentName: componentName,
	}
}

func (l standardLogger) Log(ctx context.Context, traceLabel string, severity Severity, format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "\n%s - %s - %s - %s\n", l.componentName, traceLabel, string(severity), fmt.Sprintf(format, a...))
}
