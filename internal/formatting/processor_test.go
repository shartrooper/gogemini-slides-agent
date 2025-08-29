package formatting

import (
	"reflect"
	"testing"
)

func TestTextProcessor_ParseMarkup(t *testing.T) {
	processor := NewTextProcessor()

	tests := []struct {
		name     string
		input    string
		expected []TextSegment
	}{
		{
			name:  "simple bold text",
			input: "This is **bold** text",
			expected: []TextSegment{
				{Text: "This is "},
				{Text: "bold", IsBold: true},
				{Text: " text"},
			},
		},
		{
			name:  "multiple bold sections",
			input: "**First** and **second** bold",
			expected: []TextSegment{
				{Text: "First", IsBold: true},
				{Text: " and "},
				{Text: "second", IsBold: true},
				{Text: " bold"},
			},
		},
		{
			name:  "bullet point",
			input: "• This is a bullet point",
			expected: []TextSegment{
				{Text: "This is a bullet point", IsBullet: true, Level: 0},
			},
		},
		{
			name:  "sub-bullet point",
			input: "  ◦ This is a sub-bullet",
			expected: []TextSegment{
				{Text: "This is a sub-bullet", IsBullet: true, Level: 1},
			},
		},
		{
			name:  "bullet with bold",
			input: "• **Key point** with details",
			expected: []TextSegment{
				{Text: "Key point", IsBold: true, IsBullet: true, Level: 0},
				{Text: " with details", IsBullet: true, Level: 0},
			},
		},
		{
			name:  "complex mixed content",
			input: "**Machine Learning** overview:\n• **Supervised** learning\n  ◦ Classification tasks\n• **Unsupervised** learning",
			expected: []TextSegment{
				{Text: "Machine Learning", IsBold: true},
				{Text: " overview:"},
				{Text: "\n"},
				{Text: "Supervised", IsBold: true, IsBullet: true, Level: 0},
				{Text: " learning", IsBullet: true, Level: 0},
				{Text: "\n"},
				{Text: "Classification tasks", IsBullet: true, Level: 1},
				{Text: "\n"},
				{Text: "Unsupervised", IsBold: true, IsBullet: true, Level: 0},
				{Text: " learning", IsBullet: true, Level: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.ParseMarkup(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseMarkup() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestTextProcessor_CleanText(t *testing.T) {
	processor := NewTextProcessor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove bold markup",
			input:    "This is **bold** text",
			expected: "This is bold text",
		},
		{
			name:     "remove bullet markers",
			input:    "• First point\n  ◦ Sub point\n• Second point",
			expected: "First point\nSub point\nSecond point",
		},
		{
			name:     "complex mixed content",
			input:    "**AI Ethics** involves:\n• **Bias prevention** in algorithms\n  ◦ **Fairness** metrics\n• **Privacy protection**",
			expected: "AI Ethics involves:\nBias prevention in algorithms\nFairness metrics\nPrivacy protection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.CleanText(tt.input)
			if result != tt.expected {
				t.Errorf("CleanText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTextProcessor_ToSlidesRequests(t *testing.T) {
	processor := NewTextProcessor()
	objectID := "test_object_id"

	tests := []struct {
		name     string
		segments []TextSegment
		expected struct {
			plainText    string
			boldRanges   int
			bulletRanges int
		}
	}{
		{
			name: "simple bold text",
			segments: []TextSegment{
				{Text: "This is "},
				{Text: "bold", IsBold: true},
				{Text: " text"},
			},
			expected: struct {
				plainText    string
				boldRanges   int
				bulletRanges int
			}{
				plainText:    "This is bold text",
				boldRanges:   1,
				bulletRanges: 0,
			},
		},
		{
			name: "bullet with bold",
			segments: []TextSegment{
				{Text: "Key point", IsBold: true, IsBullet: true, Level: 0},
				{Text: " details", IsBullet: true, Level: 0},
			},
			expected: struct {
				plainText    string
				boldRanges   int
				bulletRanges int
			}{
				plainText:    "Key point details",
				boldRanges:   1,
				bulletRanges: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := processor.ToSlidesRequests(tt.segments, objectID)

			// Check that we have the expected number of requests
			expectedRequests := 1 + tt.expected.boldRanges + tt.expected.bulletRanges // 1 for InsertText
			if len(requests) != expectedRequests {
				t.Errorf("ToSlidesRequests() returned %d requests, want %d", len(requests), expectedRequests)
			}

			// Check InsertText request
			if requests[0].InsertText == nil {
				t.Error("First request should be InsertText")
			} else if requests[0].InsertText.Text != tt.expected.plainText {
				t.Errorf("InsertText.Text = %q, want %q", requests[0].InsertText.Text, tt.expected.plainText)
			}

			// Count bold and bullet requests
			boldCount := 0
			bulletCount := 0
			for _, req := range requests[1:] {
				if req.UpdateTextStyle != nil {
					boldCount++
				}
				if req.CreateParagraphBullets != nil {
					bulletCount++
				}
			}

			if boldCount != tt.expected.boldRanges {
				t.Errorf("Found %d bold requests, want %d", boldCount, tt.expected.boldRanges)
			}
			if bulletCount != tt.expected.bulletRanges {
				t.Errorf("Found %d bullet requests, want %d", bulletCount, tt.expected.bulletRanges)
			}
		})
	}
}

func TestTextProcessor_Integration(t *testing.T) {
	processor := NewTextProcessor()
	input := "**Machine Learning** revolutionizes healthcare:\n• **Diagnostic accuracy** - 95% improvement\n• **Drug discovery** - Reduces time by **40%**"

	// Parse markup
	segments := processor.ParseMarkup(input)

	// Convert to Slides requests
	requests := processor.ToSlidesRequests(segments, "test_id")

	// Verify we get expected request types
	hasInsertText := false
	hasBoldFormatting := false
	hasBulletFormatting := false

	for _, req := range requests {
		if req.InsertText != nil {
			hasInsertText = true
		}
		if req.UpdateTextStyle != nil && req.UpdateTextStyle.Style != nil && req.UpdateTextStyle.Style.Bold {
			hasBoldFormatting = true
		}
		if req.CreateParagraphBullets != nil {
			hasBulletFormatting = true
		}
	}

	if !hasInsertText {
		t.Error("Expected InsertText request")
	}
	if !hasBoldFormatting {
		t.Error("Expected bold formatting request")
	}
	if !hasBulletFormatting {
		t.Error("Expected bullet formatting request")
	}

	// Verify clean text removes markup
	cleanText := processor.CleanText(input)
	expectedClean := "Machine Learning revolutionizes healthcare:\nDiagnostic accuracy - 95% improvement\nDrug discovery - Reduces time by 40%"
	if cleanText != expectedClean {
		t.Errorf("CleanText() = %q, want %q", cleanText, expectedClean)
	}
}

// Benchmark tests for performance
func BenchmarkParseMarkup(b *testing.B) {
	processor := NewTextProcessor()
	input := "**Machine Learning** revolutionizes healthcare:\n• **Diagnostic accuracy** - 95% improvement\n• **Drug discovery** - Reduces time by **40%**\n  ◦ **Protein folding** prediction\n  ◦ **Molecular simulation** advances"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.ParseMarkup(input)
	}
}

func BenchmarkToSlidesRequests(b *testing.B) {
	processor := NewTextProcessor()
	segments := []TextSegment{
		{Text: "Machine Learning", IsBold: true},
		{Text: " revolutionizes healthcare:\n"},
		{Text: "Diagnostic accuracy", IsBold: true, IsBullet: true, Level: 0},
		{Text: " - 95% improvement\n", IsBullet: true, Level: 0},
		{Text: "Drug discovery", IsBold: true, IsBullet: true, Level: 0},
		{Text: " - Reduces time by ", IsBullet: true, Level: 0},
		{Text: "40%", IsBold: true, IsBullet: true, Level: 0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.ToSlidesRequests(segments, "test_id")
	}
}
