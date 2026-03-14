package tracing

import (
	"context"
	"io"
)

type TracerConfig struct {
	ServiceName string
	AgentHost   string
	AgentPort   string
}

func InitTracer(cfg TracerConfig) (io.Closer, error) {
	return nil, nil
}

func StartSpan(ctx context.Context, operationName string) (context.Context, interface{}) {
	return ctx, nil
}

func FinishSpan(span interface{}) {
}

func LogError(span interface{}, err error) {
}
