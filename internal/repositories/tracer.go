package repositories

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("github.com/KShekhurin/blog-go/internal/repositories")
