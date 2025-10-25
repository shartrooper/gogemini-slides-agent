package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"gogemini-practices/internal/presentation"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"google.golang.org/api/slides/v1"
	genai "google.golang.org/genai"
)

type DataPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type Dataset struct {
	Title  string      `json:"title,omitempty"`
	Unit   string      `json:"unit,omitempty"`
	Type   string      `json:"type,omitempty"` // timeseries | category | comparison
	Points []DataPoint `json:"points"`
}

type TopicSummary struct {
	Topic        string   `json:"topic"`
	Summary      string   `json:"summary"`
	Quantifiable bool     `json:"quantifiable,omitempty"`
	Dataset      *Dataset `json:"dataset,omitempty"`
}

type Meta struct {
	Model        string `json:"model"`
	LatencyMs    int64  `json:"latency_ms"`
	PromptTokens int32  `json:"prompt_tokens,omitempty"`
	OutputTokens int32  `json:"output_tokens,omitempty"`
	TotalTokens  int32  `json:"total_tokens,omitempty"`
}

type Response struct {
	Topics []TopicSummary `json:"topics"`
	Meta   Meta           `json:"meta"`
}

func main() {
	_ = godotenv.Load()

	subject := flag.String("subject", "", "Presentation subject (required)")
	audience := flag.String("audience", "", "Intended audience (optional)")
	tone := flag.String("tone", "", "Tone/style (optional)")
	maxTopics := flag.Int("max", 5, "Max topics (<=5)")
	model := flag.String("model", "gemini-2.0-flash", "Gemini model to use")
	presentationID := flag.String("presentation-id", "", "Google Slides presentation ID to edit (optional)")
	flag.Parse()

	if *subject == "" {
		log.Fatal("--subject is required")
	}
	if *maxTopics <= 0 || *maxTopics > 5 {
		v := 5
		maxTopics = &v
	}

	apiKey := firstNonEmpty(os.Getenv("GOOGLE_API_KEY"), os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		log.Fatal("Set GOOGLE_API_KEY or GEMINI_API_KEY")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		log.Fatal(err)
	}

	prompt := buildPrompt(*subject, *audience, *tone, *maxTopics)
	started := time.Now()
	res, err := client.Models.GenerateContent(ctx, *model, genai.Text(prompt), nil)
	if err != nil {
		log.Fatal(err)
	}
	used := res

	var topics []TopicSummary
	cleaned := extractJSON(res.Text())
	if err := json.Unmarshal([]byte(cleaned), &topics); err != nil {
		retryPrompt := prompt + "\n\nReturn STRICT JSON only. No code fences. No backticks."
		res2, err2 := client.Models.GenerateContent(ctx, *model, genai.Text(retryPrompt), nil)
		if err2 != nil {
			log.Fatal(err2)
		}
		cleaned2 := extractJSON(res2.Text())
		if err := json.Unmarshal([]byte(cleaned2), &topics); err != nil {
			log.Fatalf("invalid JSON from model: %v\nraw: %s", err, res2.Text())
		}
		used = res2
	}

	if len(topics) > *maxTopics {
		topics = topics[:*maxTopics]
	}

	for i := range topics {
		topics[i].Topic = strings.TrimSpace(topics[i].Topic)
		topics[i].Summary = strings.TrimSpace(topics[i].Summary)
		sanitizeDataset(&topics[i])
	}

	meta := Meta{Model: *model, LatencyMs: time.Since(started).Milliseconds()}
	if used != nil && used.UsageMetadata != nil {
		meta.PromptTokens = int32(used.UsageMetadata.PromptTokenCount)
		meta.OutputTokens = int32(used.UsageMetadata.CandidatesTokenCount)
		meta.TotalTokens = int32(used.UsageMetadata.TotalTokenCount)
	}

	outObj := Response{Topics: topics, Meta: meta}
	out, err := json.MarshalIndent(outObj, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))

	if *presentationID != "" {
		credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if credsPath == "" {
			log.Println("GOOGLE_APPLICATION_CREDENTIALS not set; skipping Slides editing")
			return
		}
		credsBytes, err := os.ReadFile(credsPath)
		if err != nil {
			log.Printf("read creds: %v", err)
			return
		}
		userEmail := os.Getenv("GOOGLE_IMPERSONATE_USER")

		var slidesSvc *slides.Service
		var sheetsSvc *sheets.Service

		if userEmail != "" {
			config, err := google.JWTConfigFromJSON(credsBytes, slides.PresentationsScope, drive.DriveScope, sheets.SpreadsheetsScope)
			if err != nil {
				log.Printf("google.JWTConfigFromJSON: %v", err)
				return
			}
			config.Subject = userEmail
			client := config.Client(ctx)
			slidesSvc, err = slides.NewService(ctx, option.WithHTTPClient(client))
			if err != nil {
				log.Printf("slides.NewService: %v", err)
				return
			}
			sheetsSvc, err = sheets.NewService(ctx, option.WithHTTPClient(client))
			if err != nil {
				log.Printf("sheets.NewService: %v", err)
				return
			}
		} else {
			opts := []option.ClientOption{
				option.WithCredentialsJSON(credsBytes),
				option.WithScopes(slides.PresentationsScope, drive.DriveScope, sheets.SpreadsheetsScope),
			}
			slidesSvc, err = slides.NewService(ctx, opts...)
			if err != nil {
				log.Printf("slides.NewService: %v", err)
				return
			}
			sheetsSvc, err = sheets.NewService(ctx, opts...)
			if err != nil {
				log.Printf("sheets.NewService: %v", err)
				return
			}
		}

		// Map topics to RichTopic (with optional dataset) and write with charts
		var rich []presentation.RichTopic
		for _, t := range topics {
			rt := presentation.RichTopic{Title: t.Topic, Summary: t.Summary}
			if t.Dataset != nil && len(t.Dataset.Points) > 0 {
				cd := &presentation.ChartDataset{Title: t.Dataset.Title, Unit: t.Dataset.Unit, Type: t.Dataset.Type}
				for _, p := range t.Dataset.Points {
					cd.Points = append(cd.Points, struct {
						Label string
						Value float64
					}{Label: p.Label, Value: p.Value})
				}
				rt.Dataset = cd
			}
			rich = append(rich, rt)
		}
		if err := presentation.WriteTopicsWithCharts(ctx, slidesSvc, sheetsSvc, *presentationID, rich); err != nil {
			log.Printf("WriteTopicsWithCharts: %v", err)
		}
		return
	}
}

func buildPrompt(subject, audience, tone string, max int) string {
	var b strings.Builder
	b.WriteString("You are an expert presentation planner.\n")
	b.WriteString("Return JSON only, matching this schema: ")
	b.WriteString(`[{"topic":"string","summary":"string","quantifiable":boolean,"dataset":{"title":"string","unit":"string","type":"timeseries|category|comparison","points":[{"label":"string","value":number}]}}]`)
	b.WriteString("\nRules: Max ")
	b.WriteString(fmt.Sprintf("%d", max))
	b.WriteString(" items. Each summary <= 280 chars. No extra fields. No prose outside JSON. Do not use code fences or backticks.\n\n")

	b.WriteString("FORMATTING INSTRUCTIONS:\n")
	b.WriteString("- Use **text** to mark key information that should be bold\n")
	b.WriteString("- Use • for main bullet points of core information\n")
	b.WriteString("- Use   ◦ for sub-bullets (indented points)\n")
	b.WriteString("- Keep summaries <= 280 chars including markup\n\n")

	b.WriteString("QUANTIFIABILITY & DATASET RULES:\n")
	b.WriteString("- Set quantifiable=true only if the subject can be represented with numeric data points.\n")
	b.WriteString("- If quantifiable=true, include a compact dataset with <= 12 points that supports a chart.\n")
	b.WriteString("- Choose dataset.type: 'timeseries' for time-based, 'category' for categorical bars, 'comparison' for A vs B.\n")
	b.WriteString("- Use clear 'label' strings (e.g., '1990s', 'Q1 2024', 'Ferrari', 'Williams').\n")
	b.WriteString("- 'value' must be a number (no symbols). Include 'unit' if relevant (%, people, points).\n\n")

	b.WriteString("Example summary format:\n")
	b.WriteString(`"**Machine Learning** revolutionizes healthcare through:\n• **Diagnostic accuracy** - 95% improvement in imaging\n• **Drug discovery** - Reduces time by **40%**\n  ◦ Protein folding prediction\n  ◦ Molecular simulation"`)
	b.WriteString("\n\n")

	b.WriteString("Example quantifiable subjects:\n")
	b.WriteString("- Population growth of New York City by decades → timeseries (unit: people)\n")
	b.WriteString("- Ferrari vs Williams F1 pilots performance in the last grand prix → comparison (unit: points)\n")
	b.WriteString("- Evolution of videogame company Steam → timeseries (unit: MAU or revenue)\n\n")

	b.WriteString("Inputs:\n")
	b.WriteString("Subject: ")
	b.WriteString(subject)
	if audience != "" {
		b.WriteString("\nAudience: ")
		b.WriteString(audience)
	}
	if tone != "" {
		b.WriteString("\nTone: ")
		b.WriteString(tone)
	}
	b.WriteString("\nTask: Propose the most relevant topics and a concise summary for each using the formatting markup above. Decide if each is quantifiable and include a compact dataset when appropriate.")
	return b.String()
}

func sanitizeDataset(t *TopicSummary) {
	if t == nil || t.Dataset == nil {
		return
	}
	const maxPoints = 20
	if len(t.Dataset.Points) > maxPoints {
		t.Dataset.Points = t.Dataset.Points[:maxPoints]
	}
	valid := make([]DataPoint, 0, len(t.Dataset.Points))
	for _, p := range t.Dataset.Points {
		label := strings.TrimSpace(p.Label)
		if label == "" {
			continue
		}
		if math.IsNaN(p.Value) || math.IsInf(p.Value, 0) {
			continue
		}
		valid = append(valid, DataPoint{Label: label, Value: p.Value})
	}
	t.Dataset.Points = valid
	if len(t.Dataset.Points) == 0 {
		t.Dataset = nil
		t.Quantifiable = false
		return
	}
	t.Quantifiable = true
	switch strings.ToLower(strings.TrimSpace(t.Dataset.Type)) {
	case "timeseries", "category", "comparison":
	default:
		t.Dataset.Type = "category"
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func extractJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}
	if i := strings.IndexAny(s, "[{"); i != -1 {
		s = s[i:]
	}

	if strings.HasPrefix(s, "[") {
		if j := strings.LastIndex(s, "]"); j != -1 {
			return strings.TrimSpace(s[:j+1])
		}
	}
	if strings.HasPrefix(s, "{") {
		if j := strings.LastIndex(s, "}"); j != -1 {
			return strings.TrimSpace(s[:j+1])
		}
	}
	return s
}
