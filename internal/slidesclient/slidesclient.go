package slidesclient

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/api/option"
	"google.golang.org/api/slides/v1"
)

func NewFromJSON(ctx context.Context, serviceAccountJSON []byte) (*slides.Service, error) {
	if len(serviceAccountJSON) == 0 {
		return nil, fmt.Errorf("empty service account JSON")
	}
	svc, err := slides.NewService(
		ctx,
		option.WithCredentialsJSON(serviceAccountJSON),
		option.WithScopes(slides.PresentationsScope),
	)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func NewFromFile(ctx context.Context, path string) (*slides.Service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credentials file: %w", err)
	}
	return NewFromJSON(ctx, data)
}
