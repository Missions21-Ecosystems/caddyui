package basepath

import "context"

type key struct{}

func WithBasePath(ctx context.Context, p string) context.Context {
	return context.WithValue(ctx, key{}, p)
}

func Get(ctx context.Context) string {
	if v, ok := ctx.Value(key{}).(string); ok {
		return v
	}
	return ""
}
