package templates

import (
	"caddyui/internal/basepath"
	"context"
)

// bp returns the browser-side base path prefix for the current request.
func bp(ctx context.Context) string {
	return basepath.Get(ctx)
}
