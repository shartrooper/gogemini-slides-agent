package slidesclient

import (
	"context"
	"os"
	"strings"
	"testing"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/slides/v1"
)

func TestServiceAccountConnection(t *testing.T) {
	ctx := context.Background()

	if v := os.Getenv("TEST_SA_JSON"); v != "" {
		data := resolveEnvValueToJSON(t, v)
		assertTokenAndClient(t, ctx, data)
		return
	}

	if raw := os.Getenv("SLIDES_SA_JSON"); raw != "" {
		data := []byte(raw)
		// If SLIDES_SA_JSON looks like a file path, read it
		if !strings.HasPrefix(strings.TrimSpace(raw), "{") {
			b, err := os.ReadFile(raw)
			if err == nil {
				data = b
			}
		}
		assertTokenAndClient(t, ctx, data)
		return
	}

	if credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); credPath != "" {
		b, err := os.ReadFile(credPath)
		if err != nil {
			t.Fatalf("read creds: %v", err)
		}
		assertTokenAndClient(t, ctx, b)
		return
	}

	t.Skip("Set TEST_SA_JSON (path or JSON), SLIDES_SA_JSON (JSON or path), or GOOGLE_APPLICATION_CREDENTIALS (path)")
}

func resolveEnvValueToJSON(t *testing.T, val string) []byte {
	if strings.HasPrefix(strings.TrimSpace(val), "{") {
		return []byte(val)
	}
	b, err := os.ReadFile(val)
	if err != nil {
		t.Fatalf("read creds: %v", err)
	}
	return b
}

func assertTokenAndClient(t *testing.T, ctx context.Context, data []byte) {
	creds, err := google.CredentialsFromJSON(ctx, data, slides.PresentationsScope)
	if err != nil {
		t.Fatalf("CredentialsFromJSON: %v", err)
	}
	if _, err := creds.TokenSource.Token(); err != nil {
		t.Fatalf("Token(): %v", err)
	}
	if _, err := NewFromJSON(ctx, data); err != nil {
		t.Fatalf("NewFromJSON: %v", err)
	}
}
