package tenant

import (
	"context"
	"errors"
)

var ErrMissing = errors.New("tenant missing from authenticated context")

type key struct{}

func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key{}, id)
}

func Require(ctx context.Context) (string, error) {
	id, ok := ctx.Value(key{}).(string)
	if !ok || id == "" {
		return "", ErrMissing
	}
	return id, nil
}
