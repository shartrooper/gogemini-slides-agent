package formatting

import (
	"regexp"
	"strings"

	"google.golang.org/api/slides/v1"
)

// TextSegment represents a piece of text with formatting information
type TextSegment struct {
	Text     string
	IsBold   bool
	IsBullet bool
	Level    int // 0=main bullet, 1=sub-bullet
}

// TextProcessor handles conversion from custom markup to Google Slides formatting
type TextProcessor struct {
	boldPattern      *regexp.Regexp
	bulletPattern    *regexp.Regexp
	subBulletPattern *regexp.Regexp
}

// NewTextProcessor creates a new text processor with compiled regex patterns
func NewTextProcessor() *TextProcessor {
	return &TextProcessor{
		boldPattern:      regexp.MustCompile(`\*\*(.*?)\*\*`),
		bulletPattern:    regexp.MustCompile(`^• (.*)$`),
		subBulletPattern: regexp.MustCompile(`^  ◦ (.*)$`),
	}
}

// ParseMarkup converts custom markup text into structured segments
func (tp *TextProcessor) ParseMarkup(text string) []TextSegment {
	var segments []TextSegment
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		// Check if line is a bullet point
		if tp.bulletPattern.MatchString(line) {
			content := tp.bulletPattern.ReplaceAllString(line, "$1")
			segments = append(segments, tp.parseBoldInText(content, true, 0)...)
		} else if tp.subBulletPattern.MatchString(line) {
			content := tp.subBulletPattern.ReplaceAllString(line, "$1")
			segments = append(segments, tp.parseBoldInText(content, true, 1)...)
		} else {
			// Regular text, check for bold markup
			segments = append(segments, tp.parseBoldInText(line, false, 0)...)
		}

		// Add newline segment except for last line
		if line != lines[len(lines)-1] {
			segments = append(segments, TextSegment{Text: "\n"})
		}
	}

	return segments
}

// parseBoldInText extracts bold markup from text and creates segments
func (tp *TextProcessor) parseBoldInText(text string, isBullet bool, level int) []TextSegment {
	var segments []TextSegment
	lastEnd := 0

	matches := tp.boldPattern.FindAllStringSubmatchIndex(text, -1)

	for _, match := range matches {
		start, end := match[0], match[1]
		boldStart, boldEnd := match[2], match[3]

		// Add text before bold
		if start > lastEnd {
			segments = append(segments, TextSegment{
				Text:     text[lastEnd:start],
				IsBullet: isBullet,
				Level:    level,
			})
		}

		// Add bold text
		segments = append(segments, TextSegment{
			Text:     text[boldStart:boldEnd],
			IsBold:   true,
			IsBullet: isBullet,
			Level:    level,
		})

		lastEnd = end
	}

	// Add remaining text
	if lastEnd < len(text) {
		segments = append(segments, TextSegment{
			Text:     text[lastEnd:],
			IsBullet: isBullet,
			Level:    level,
		})
	}

	return segments
}

// ToSlidesRequests converts text segments to Google Slides API requests
func (tp *TextProcessor) ToSlidesRequests(segments []TextSegment, objectID string) []*slides.Request {
	var requests []*slides.Request

	// First, build the plain text and collect formatting info
	plainText := ""
	var boldRanges []struct{ start, end int }
	var bulletRanges []struct{ start, end, level int }

	currentPos := 0
	bulletStart := -1
	currentBulletLevel := -1

	for _, segment := range segments {
		segmentStart := currentPos
		segmentEnd := currentPos + len(segment.Text)

		plainText += segment.Text

		// Track bold ranges
		if segment.IsBold {
			boldRanges = append(boldRanges, struct{ start, end int }{segmentStart, segmentEnd})
		}

		// Track bullet ranges
		if segment.IsBullet {
			if bulletStart == -1 {
				bulletStart = segmentStart
				currentBulletLevel = segment.Level
			}
		} else if bulletStart != -1 {
			// End of bullet section
			bulletRanges = append(bulletRanges, struct{ start, end, level int }{
				bulletStart, currentPos, currentBulletLevel,
			})
			bulletStart = -1
		}

		currentPos = segmentEnd
	}

	// Handle final bullet range
	if bulletStart != -1 {
		bulletRanges = append(bulletRanges, struct{ start, end, level int }{
			bulletStart, currentPos, currentBulletLevel,
		})
	}

	// Insert the plain text
	requests = append(requests, &slides.Request{
		InsertText: &slides.InsertTextRequest{
			ObjectId:       objectID,
			InsertionIndex: 0,
			Text:           plainText,
		},
	})

	// Apply bold formatting
	for _, boldRange := range boldRanges {
		startIdx := int64(boldRange.start)
		endIdx := int64(boldRange.end)
		requests = append(requests, &slides.Request{
			UpdateTextStyle: &slides.UpdateTextStyleRequest{
				ObjectId: objectID,
				Style: &slides.TextStyle{
					Bold: true,
				},
				TextRange: &slides.Range{
					StartIndex: &startIdx,
					EndIndex:   &endIdx,
				},
			},
		})
	}

	// Apply bullet formatting
	for _, bulletRange := range bulletRanges {
		bulletPreset := "BULLET_DISC_CIRCLE_SQUARE"
		if bulletRange.level == 1 {
			bulletPreset = "BULLET_HOLLOW_CIRCLE_SQUARE"
		}

		startIdx := int64(bulletRange.start)
		endIdx := int64(bulletRange.end)
		requests = append(requests, &slides.Request{
			CreateParagraphBullets: &slides.CreateParagraphBulletsRequest{
				ObjectId: objectID,
				TextRange: &slides.Range{
					StartIndex: &startIdx,
					EndIndex:   &endIdx,
				},
				BulletPreset: bulletPreset,
			},
		})
	}

	return requests
}

// CleanText removes all markup and returns plain text
func (tp *TextProcessor) CleanText(text string) string {
	// Remove bold markup
	cleaned := tp.boldPattern.ReplaceAllString(text, "$1")

	// Remove bullet markers
	lines := strings.Split(cleaned, "\n")
	for i, line := range lines {
		if tp.bulletPattern.MatchString(line) {
			lines[i] = tp.bulletPattern.ReplaceAllString(line, "$1")
		} else if tp.subBulletPattern.MatchString(line) {
			lines[i] = tp.subBulletPattern.ReplaceAllString(line, "$1")
		}
	}

	return strings.Join(lines, "\n")
}
