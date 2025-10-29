### Edge Cases and Guardrails (QA Flowchart)

```mermaid
graph TD;
  A[Start];
  B[Normalize and sanitize inputs];
  C{Numeric only inputs};
  D{Gibberish heuristic};
  E[Apply length limits: subject 120, audience 160, tone 60];
  F[Strip adversarial phrases];
  F1{LLM classifier TRUE or FALSE};
  G[Build prompt with safety, schema, formatting rules];
  H[Call Gemini GenerateContent];
  I{Valid JSON};
  J[Retry with STRICT JSON];
  K[Clamp topics to max up to 5 and sanitize datasets];
  L{Presentation ID provided};
  M{Sheet ID provided};
  N[Init Slides and Sheets clients];
  O[Delete all existing slides];
  P[Spreadsheet cleanup: delete CHART tabs and Data_N tabs];
  Q{For each topic};
  R[Create Title and Image slide];
  R1{CSE configured};
  R2[Search images up to five];
  R3{Any image found};
  R4[Validate image via HEAD];
  R5[Insert image];
  R6[Use fallback image URL];
  S[Create Summary slide];
  T{Dataset exists};
  V[Write Data_N sheet, add chart tab, embed chart];
  W[Commit BatchUpdate];
  X1[Exit: numeric only input];
  X2[Exit: gibberish input];
  X3[Exit: model flagged inputs];
  Y1[Print JSON only and exit];
  Y2[Log and exit: sheet ID required];
  Z[End];

  A --> B;
  B --> C;
  C -- Yes --> X1;
  C -- No --> D;
  D -- Yes --> X2;
  D -- No --> E;
  E --> F;
  F --> F1;
  F1 -- TRUE --> X3;
  F1 -- FALSE --> G;
  G --> H;
  H --> I;
  I -- No --> J;
  J --> I;
  I -- Yes --> K;
  K --> L;
  L -- No --> Y1;
  L -- Yes --> M;
  M -- No --> Y2;
  M -- Yes --> N;
  N --> O;
  O --> P;
  P --> Q;
  Q --> R;
  R --> R1;
  R1 -- Yes --> R2;
  R2 --> R3;
  R3 -- Yes --> R4;
  R4 --> R5;
  R3 -- No --> R6;
  R6 --> R5;
  R1 -- No --> R5;
  R5 --> S;
  S --> T;
  T -- No --> Q;
  T -- Yes --> V;
  V --> Q;
  Q --> W;
  W --> Z;
```

### QA Edge Cases and Expected Outcomes

- **Numeric-only subject/audience/tone**: CLI exits with error. No model call.
- **Gibberish (heuristic)**: CLI exits with error. No model call.
- **LLM classifier TRUE**: CLI exits with error. No generation.
- **Length over limits**: Inputs are truncated (subject=120, audience=160, tone=60). Generation proceeds.
- **Prompt-injection phrases present**: Phrases are stripped; prompt includes safety note. Generation proceeds.
- **Non-JSON model output**: One retry with “STRICT JSON” reminder; on success, proceed; otherwise exit with parse error.
- **Topics > max**: Truncated to `--max` (≤5).

### Slides and Sheets behavior to test

- **Full slide wipe**: All existing slides are deleted up front. Expect only newly generated slides in strict order per topic (Title+Image → Summary → Chart).
- **Spreadsheet cleanup**: Deletes CHART tabs and any `Data_` tabs; ensures at least one grid sheet remains. Per-topic write clears `A:Z` before writing values.

### Image search and fallback cases

- **CSE unset or empty results**: Use fallback image URL; if fallback unreachable, skip image.
- **Invalid image URL (non-HTTPS or broken)**: HEAD check fails → use fallback image URL.
- **Fallback URL**: Defaults to a valid HTTPS placeholder; override with `--default-image-url` or `DEFAULT_IMAGE_URL`.
- **Param variations**: QA may vary `imgSize`, `imgType`, `imgColorType`, `imgDominant`, `img-rights`, `img-safe` and confirm request formation (max 5 results).

### Required IDs and client setup

- **Missing `--sheet-id` when `--presentation-id` is set**: Log and exit after printing JSON.
- **No credentials** (`GOOGLE_APPLICATION_CREDENTIALS` unset): Log and exit after JSON.
- **Impersonation optional**: If set but unauthorized, expect an auth error; if unset, service account is used.

### Known transient/service edge cases
- **Classifier 429/RESOURCE_EXHAUSTED**: One backoff retry (~350ms). On repeated failure, log warning and continue generation.
- **Slides/Sheets transient 429/5xx**: Surface error; QA may simulate to verify error logging and no partial state outside slide wipe step.


