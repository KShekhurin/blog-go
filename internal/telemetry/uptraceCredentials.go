package telemetry

import "context"

type uptraceCredentials struct {
	dsn string
}

func (c uptraceCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"uptrace-dsn": c.dsn,
	}, nil
}

func (c uptraceCredentials) RequireTransportSecurity() bool {
	return false
}
