## gogemini-practices

Minimal Go CLI that:
- Generates up to five summarized topics using Gemini and prints strict JSON
- Optionally edits an existing Google Slides deck and writes one slide per topic

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

- Generate and write to an existing Slides file:
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
    { "topic": "string", "summary": "string" }
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

### Tests
Unit test validates service account credentials by fetching a token and initializing a Slides client.

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

### Notes
- Topics are capped at 5; summaries trimmed.
- If the model wraps JSON in code fences, the program strips them and retries once if parsing fails.
- For Slides editing, plain text is used (title + body per topic) with a blank layout.

