package picturegen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func TestFlashPicgen_GeneratesImageToTemp(t *testing.T) {
	if err := godotenv.Load(); err == nil {
		t.Log("Loaded .env from current directory")
	} else {
		t.Logf(".env not loaded from current dir (ok): %v", err)
		// Try project root: tests run from package dir, root is two levels up
		if err2 := godotenv.Load(filepath.Join("..", "..", ".env")); err2 == nil {
			t.Log("Loaded .env from project root")
		} else {
			t.Logf(".env not loaded from project root (ok): %v", err2)
		}
	}
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey != "" {
			t.Log("Using GEMINI_API_KEY from environment")
		}
	} else {
		t.Log("Using GOOGLE_API_KEY from environment")
	}
	if apiKey == "" {
		t.Skip("API key not set; set GOOGLE_API_KEY or GEMINI_API_KEY to run this test")
	}

	ctx := context.Background()
	prompt := "Create a picture of a nano banana dish in a fancy restaurant with a Gemini theme"
	t.Logf("Prompt: %q", prompt)
	t.Log("Calling FlashPicgen with gemini-2.5-flash-image-preview ...")

	data, err := FlashPicgen(ctx, prompt, apiKey)
	if err != nil {
		// Skip gracefully on quota or rate-limit errors
		errStr := err.Error()
		if strings.Contains(errStr, "RESOURCE_EXHAUSTED") || strings.Contains(errStr, "429") || strings.Contains(errStr, "quota") {
			t.Skipf("Skipping due to quota/rate limit: %v", err)
		}
		t.Fatalf("FlashPicgen returned error: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("no image data returned")
	}
	t.Logf("Received image bytes: %d", len(data))

	// Write to a temp folder within the repo root (testing unit root)
	tempDir := filepath.Join("..", "..", "tmp_test_output")
	t.Logf("Creating temp dir: %s", tempDir)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	outPath := filepath.Join(tempDir, "gemini_generated_image.png")
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		t.Fatalf("failed to write image: %v", err)
	}
	t.Logf("Wrote image to: %s", outPath)
}
