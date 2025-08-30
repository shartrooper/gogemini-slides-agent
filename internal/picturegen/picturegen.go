package picturegen

import (
	"context"
	"errors"

	genai "google.golang.org/genai"
)

// FlashPicgen generates an image using the Gemini image preview model.
// It returns the raw bytes of the first image produced by the model.
func FlashPicgen(ctx context.Context, prompt string, apiKey string) ([]byte, error) {
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if apiKey == "" {
		return nil, errors.New("apiKey is required")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		return nil, err
	}

	res, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash-image-preview",
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		return nil, err
	}

	if res == nil || len(res.Candidates) == 0 || res.Candidates[0] == nil || res.Candidates[0].Content == nil {
		return nil, errors.New("no candidates returned from model")
	}

	for _, part := range res.Candidates[0].Content.Parts {
		if part.InlineData != nil && len(part.InlineData.Data) > 0 {
			return part.InlineData.Data, nil
		}
	}

	return nil, errors.New("no image data returned from model")
}
