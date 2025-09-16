## gogemini-practices

Minimal Go CLI and helpers that:
- Generates up to five summarized topics using Gemini and prints strict JSON
- Emits lightweight formatting markup in the summaries (bold + bullets)
- Optionally edits an existing Google Slides deck and writes one slide per topic, converting the markup into Slides formatting (bold text + bullet lists)
- Includes an image utility to generate a picture via the Gemini image preview model

### Requirements
- Go 1.24+
- Gemini API key
- Google Cloud project with Slides API enabled (for editing)

### Install
```bash
go build
```

### Configuration
- Create a `.env` file (auto-loaded):
```bash
GEMINI_API_KEY=your_gemini_key
# or
GOOGLE_API_KEY=your_gemini_key
```

- For Slides editing (service account or ADC):
```bash
# Service account JSON file path
GOOGLE_APPLICATION_CREDENTIALS=E:\path\to\credentials.json
```

### Usage
- Generate topics (JSON only):
```bash
go run . --subject "Tips for good dental hygiene" --audience "children" --tone "slightly serious"
```

- Generate and write to an existing Slides file (formatted):
```bash
go run . --subject "AI in Healthcare" --audience "Clinicians" --presentation-id YOUR_FILE_ID
```

Flags:
- `--subject` (required)
- `--audience`, `--tone` (optional)
- `--max` (default 5, capped at 5)
- `--model` (default `gemini-2.0-flash`)
- `--presentation-id` (edit existing deck)

### Output shape
```json
{
  "topics": [
    { "topic": "string", "summary": "string-with-lightweight-markup" }
  ],
  "meta": {
    "model": "gemini-2.0-flash",
    "latency_ms": 0,
    "prompt_tokens": 0,
    "output_tokens": 0,
    "total_tokens": 0
  }
}
```

The `summary` field may contain simple formatting markers (see below). When writing to Slides, these are converted into rich formatting.

### Formatting markup (LLM-guided)
The model is prompted to emit concise summaries using a tiny markup:

- `**text**` → bold key information
- `• ` at line start → main bullet
- `  ◦ ` at line start → sub-bullet (one level)

Example summary value:

```
"**Machine Learning** improves care via:\n• **Diagnostic accuracy** gains\n• **Workflow speed-ups** by **40%**\n  ◦ Triage automation"
```

When a Slides `presentation-id` is provided, the program:
- Inserts plain text into the slide text boxes
- Applies bold to ranges wrapped in `**` (markers are removed)
- Turns bullet lines into Slides bullets; `◦` becomes a sub-bullet preset

### Tests
Included tests:
- Slides client credential test (service account token + client init)
- Formatting parser and Slides request generation
- Image generation test for the Gemini image preview model (skips on missing key/quota)

Provide credentials via one of:
```bash
# Preferred for tests: path or raw JSON
set TEST_SA_JSON=./credentials.json
# or
set SLIDES_SA_JSON={...raw json...}
# or standard env
set GOOGLE_APPLICATION_CREDENTIALS=E:\path\to\credentials.json

go test ./...
```

### Image generation utility
The `internal/picturegen` package provides a helper to call `gemini-2.5-flash-image-preview` and return image bytes for a text prompt. See `internal/picturegen/picturegen_test.go` for an end-to-end example that writes a PNG under `tmp_test_output/`.

Run only this package's tests:
```bash
go test ./internal/picturegen -v
```

### Programmatic Slides writing (formatted)
The `internal/presentation` package exposes `WriteTopics(ctx, svc, presentationID, topics)` which creates slides (as needed), adds title/body text boxes, and converts markup to formatting.

Data shape:
```json
{ "Title": "string-with-markup", "Summary": "string-with-markup" }
```

You can build a `*slides.Service` using your own auth, or via `internal/slidesclient` helpers (service account JSON/file).

### Notes
- Topics are capped at 5; summaries trimmed.
- If the model wraps JSON in code fences, the program strips them and retries once if parsing fails.
- Slide layout used is `BLANK`; text boxes are positioned with fixed transforms.
- One sub-bullet level is supported (`◦`).
- Title fields also support the same markup.

