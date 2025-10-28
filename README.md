## gogemini-practices

Minimal Go CLI and helpers that:
- Generates up to five summarized topics using Gemini and prints strict JSON
- Emits lightweight formatting markup in the summaries (bold + bullets)
- Optionally edits an existing Google Slides deck and writes three slides per topic (Title+Image, Summary, Chart), converting the markup into Slides formatting (bold text + bullets)
- Embeds charts by writing data to an existing Google Sheets spreadsheet (per-topic `Data_N` tabs)
- Includes an image utility to generate a picture via the Gemini image preview model

### Requirements
- Go 1.24+
- Gemini API key
- Google Cloud project with Slides and Sheets APIs enabled (for editing)

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
GOOGLE_APPLICATION_CREDENTIALS=E:\path\to\credentials.json

# Optional: Custom Search for images
CSE_API_KEY=your_cse_key
CSE_CX=your_cse_engine_id

# Optional: default image fallback (HTTPS URL)
DEFAULT_IMAGE_URL=https://t3.ftcdn.net/jpg/05/79/68/24/360_F_579682465_CBq4AWAFmFT1otwioF5X327rCjkVICyH.jpg
```

For Slides editing you can use a service account JSON (`GOOGLE_APPLICATION_CREDENTIALS`) or Application Default Credentials.

### Usage
- Generate topics (JSON only):
```bash
go run . --subject "Tips for good dental hygiene" --audience "children" --tone "slightly serious"
```

- Generate and write to an existing Slides + Sheets (formatted, images + charts):
```bash
go run . \
  --subject "AI in Healthcare" \
  --audience "Clinicians" \
  --tone "concise" \
  --presentation-id <SLIDES_ID> \
  --sheet-id <SHEET_ID> \
  --cse-key $CSE_API_KEY --cse-cx $CSE_CX \
  --img-size large --img-type photo --img-safe active
```

Flags:
- `--subject` (required)
- `--audience`, `--tone` (optional)
- `--max` (default 5, capped at 5)
- `--model` (default `gemini-2.0-flash`)
- `--presentation-id` (edit existing deck)
- `--sheet-id` (required when `--presentation-id` is set; target spreadsheet for charts)
- Image search (optional): `--cse-key`, `--cse-cx`, `--img-size`, `--img-type`, `--img-color-type`, `--img-dominant`, `--img-rights`, `--img-safe`
- Image fallback: `--default-image-url` (HTTPS URL)

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
- Wipes all existing slides
- For each topic, creates three slides in order: Title+Image, Summary, Chart (if dataset present)
- Converts markup to formatting (bold ranges and bullets)
- Writes dataset to `Data_N` sheet tabs and embeds a chart

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

### Image search and image generation
Image search uses Google Custom Search (if configured) to fetch up to 5 candidate images per topic, scores them by topic-term match, validates via HTTPS HEAD, and inserts the best image or falls back to a default HTTPS placeholder.

The `internal/picturegen` package provides a helper to call `gemini-2.5-flash-image-preview` and return image bytes for a text prompt. See `internal/picturegen/picturegen_test.go` for an end-to-end example that writes a PNG under `tmp_test_output/`.
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

### Guardrails & edge cases
- Inputs are validated and sanitized: numeric-only detection, gibberish check, length limits, prompt-injection phrase stripping.
- A cheap LLM pre-check classifies inputs (TRUE/FALSE) for gibberish/jailbreak; TRUE aborts early.
- Non-JSON outputs trigger a single strict-JSON retry.
- See `EDGE_CASES.md` for QA flowchart and expected outcomes.

