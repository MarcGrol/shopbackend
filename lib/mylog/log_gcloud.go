package mylog

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
)

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		New = newGloudLogger
		// Disable log prefixes such as the default timestamp.
		// Prefix text prevents the message from being parsed as JSON.
		// A timestamp is added when shipping logs to Cloud Logging.
		log.SetFlags(0)
	}
}

type structuredLogger struct {
	componentName string
}

func newGloudLogger(componentName string) Logger {
	return structuredLogger{
		componentName: componentName,
	}
}

func (l structuredLogger) Log(ctx context.Context, traceLabel string, severity Severity, format string, a ...interface{}) {
	log.Println(entry{
		Component: l.componentName,
		Labels:    map[string]string{"aggregate": traceLabel},
		Trace:     ctx.Value(mycontext.CtxTraceContext{}).(string),
		Severity:  string(severity),
		Message:   l.componentName + ":" + fmt.Sprintf(format, a...),
	}.String())
}

type entry struct {
	Component string            `json:"component,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Trace     string            `json:"logging.googleapis.com/trace,omitempty"`
	Severity  string            `json:"severity,omitempty"`
	Message   string            `json:"message"`
}

func (e entry) String() string {
	out, err := json.Marshal(e)
	if err != nil {
		log.Printf("error marshalling log record: %v", err)
	}

	return string(out)
}
