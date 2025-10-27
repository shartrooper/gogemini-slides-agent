package presentation

import (
	"context"
	"fmt"

	"gogemini-practices/internal/charts"
	"gogemini-practices/internal/formatting"

	"github.com/google/uuid"
	"google.golang.org/api/sheets/v4"
	"google.golang.org/api/slides/v1"
)

type Topic struct {
	Title   string
	Summary string
}

// ChartDataset mirrors a small chart-friendly dataset.
type ChartDataset struct {
	Title  string
	Unit   string
	Type   string // timeseries | category | comparison
	Points []struct {
		Label string
		Value float64
	}
}

// RichTopic extends Topic with an optional dataset for chart embedding.
type RichTopic struct {
	Title   string
	Summary string
	Dataset *ChartDataset
}

func WriteTopics(ctx context.Context, svc *slides.Service, presentationID string, topics []Topic) error {
	if len(topics) == 0 {
		return nil
	}

	pres, err := svc.Presentations.Get(presentationID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("get presentation: %w", err)
	}

	existing := len(pres.Slides)
	need := len(topics)

	var requests []*slides.Request
	processor := formatting.NewTextProcessor()

	targetSlideIDs := make([]string, 0, need)
	for i := 0; i < need && i < existing; i++ {
		slide := pres.Slides[i]
		if slide == nil {
			continue
		}
		for _, el := range slide.PageElements {
			if el == nil || el.ObjectId == "" {
				continue
			}
			requests = append(requests, &slides.Request{DeleteObject: &slides.DeleteObjectRequest{ObjectId: el.ObjectId}})
		}
		targetSlideIDs = append(targetSlideIDs, slide.ObjectId)
	}
	for i := existing; i < need; i++ {
		slideID := fmt.Sprintf("auto_slide_%d", i)
		targetSlideIDs = append(targetSlideIDs, slideID)
		requests = append(requests, &slides.Request{CreateSlide: &slides.CreateSlideRequest{
			ObjectId:             slideID,
			SlideLayoutReference: &slides.LayoutReference{PredefinedLayout: "BLANK"},
		}})
	}

	for i := 0; i < need; i++ {
		slideID := targetSlideIDs[i]
		suffix := uuid.New().String()[:8]
		titleID := fmt.Sprintf("auto_title_%d_%s", i, suffix)
		bodyID := fmt.Sprintf("auto_body_%d_%s", i, suffix)

		// Create title text box
		requests = append(requests,
			&slides.Request{CreateShape: &slides.CreateShapeRequest{
				ObjectId:  titleID,
				ShapeType: "TEXT_BOX",
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideID,
					Size: &slides.Size{
						Width:  &slides.Dimension{Magnitude: 600, Unit: "PT"},
						Height: &slides.Dimension{Magnitude: 60, Unit: "PT"},
					},
					Transform: &slides.AffineTransform{ScaleX: 1, ScaleY: 1, TranslateX: 50, TranslateY: 50, Unit: "PT"},
				},
			}},
		)

		// Process title formatting
		titleSegments := processor.ParseMarkup(topics[i].Title)
		titleRequests := processor.ToSlidesRequests(titleSegments, titleID)
		requests = append(requests, titleRequests...)

		// Create body text box
		requests = append(requests,
			&slides.Request{CreateShape: &slides.CreateShapeRequest{
				ObjectId:  bodyID,
				ShapeType: "TEXT_BOX",
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideID,
					Size: &slides.Size{
						Width:  &slides.Dimension{Magnitude: 600, Unit: "PT"},
						Height: &slides.Dimension{Magnitude: 300, Unit: "PT"},
					},
					Transform: &slides.AffineTransform{ScaleX: 1, ScaleY: 1, TranslateX: 50, TranslateY: 130, Unit: "PT"},
				},
			}},
		)

		// Process body formatting
		bodySegments := processor.ParseMarkup(topics[i].Summary)
		bodyRequests := processor.ToSlidesRequests(bodySegments, bodyID)
		requests = append(requests, bodyRequests...)
	}

	if len(requests) == 0 {
		return nil
	}

	_, err = svc.Presentations.BatchUpdate(presentationID, &slides.BatchUpdatePresentationRequest{Requests: requests}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("batch update: %w", err)
	}
	return nil
}

// WriteTopicsWithCharts behaves like WriteTopics but also embeds a chart for any topic with a dataset.
// It requires both Slides and Sheets services.
func WriteTopicsWithCharts(ctx context.Context, slidesSvc *slides.Service, sheetsSvc *sheets.Service, spreadsheetID string, presentationID string, topics []RichTopic) error {
	if len(topics) == 0 {
		return nil
	}
	if slidesSvc == nil {
		return fmt.Errorf("slides service is nil")
	}
	if sheetsSvc == nil {
		return fmt.Errorf("sheets service is nil")
	}

	pres, err := slidesSvc.Presentations.Get(presentationID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("get presentation: %w", err)
	}

	existing := len(pres.Slides)
	need := len(topics)

	var requests []*slides.Request
	processor := formatting.NewTextProcessor()

	targetSlideIDs := make([]string, 0, need)
	for i := 0; i < need && i < existing; i++ {
		slide := pres.Slides[i]
		if slide == nil {
			continue
		}
		for _, el := range slide.PageElements {
			if el == nil || el.ObjectId == "" {
				continue
			}
			requests = append(requests, &slides.Request{DeleteObject: &slides.DeleteObjectRequest{ObjectId: el.ObjectId}})
		}
		targetSlideIDs = append(targetSlideIDs, slide.ObjectId)
	}
	for i := existing; i < need; i++ {
		slideID := fmt.Sprintf("auto_slide_%d", i)
		targetSlideIDs = append(targetSlideIDs, slideID)
		requests = append(requests, &slides.Request{CreateSlide: &slides.CreateSlideRequest{
			ObjectId:             slideID,
			SlideLayoutReference: &slides.LayoutReference{PredefinedLayout: "BLANK"},
		}})
	}

	for i := 0; i < need; i++ {
		slideID := targetSlideIDs[i]
		suffix := uuid.New().String()[:8]
		titleID := fmt.Sprintf("auto_title_%d_%s", i, suffix)
		bodyID := fmt.Sprintf("auto_body_%d_%s", i, suffix)

		requests = append(requests,
			&slides.Request{CreateShape: &slides.CreateShapeRequest{
				ObjectId:  titleID,
				ShapeType: "TEXT_BOX",
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideID,
					Size: &slides.Size{
						Width:  &slides.Dimension{Magnitude: 600, Unit: "PT"},
						Height: &slides.Dimension{Magnitude: 60, Unit: "PT"},
					},
					Transform: &slides.AffineTransform{ScaleX: 1, ScaleY: 1, TranslateX: 50, TranslateY: 50, Unit: "PT"},
				},
			}},
		)

		titleSegments := processor.ParseMarkup(topics[i].Title)
		titleRequests := processor.ToSlidesRequests(titleSegments, titleID)
		requests = append(requests, titleRequests...)

		// Body box
		requests = append(requests,
			&slides.Request{CreateShape: &slides.CreateShapeRequest{
				ObjectId:  bodyID,
				ShapeType: "TEXT_BOX",
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideID,
					Size: &slides.Size{
						Width:  &slides.Dimension{Magnitude: 600, Unit: "PT"},
						Height: &slides.Dimension{Magnitude: 300, Unit: "PT"},
					},
					Transform: &slides.AffineTransform{ScaleX: 1, ScaleY: 1, TranslateX: 50, TranslateY: 130, Unit: "PT"},
				},
			}},
		)

		bodySegments := processor.ParseMarkup(topics[i].Summary)
		bodyRequests := processor.ToSlidesRequests(bodySegments, bodyID)
		requests = append(requests, bodyRequests...)

		// If dataset present, write data to provided spreadsheet and embed the chart
		if topics[i].Dataset != nil && len(topics[i].Dataset.Points) > 0 {
			ds := charts.DatasetSpec{Title: topics[i].Dataset.Title, Unit: topics[i].Dataset.Unit, Type: topics[i].Dataset.Type}
			for _, p := range topics[i].Dataset.Points {
				ds.Points = append(ds.Points, charts.Point{Label: p.Label, Value: p.Value})
			}
			// Use a per-topic sheet title to avoid collisions
			perSheet := fmt.Sprintf("Data_%d", i+1)
			chartID, err := charts.CreateSheetsChart(ctx, sheetsSvc, spreadsheetID, perSheet, ds)
			if err != nil {
				return fmt.Errorf("create sheets chart for topic %q: %w", topics[i].Title, err)
			}
			chartObjectID := fmt.Sprintf("auto_chart_%d_%s", i, suffix)
			// Position near the right side, below title; EMU units as in examples
			embed := charts.BuildEmbedRequests(spreadsheetID, chartID, slideID, chartObjectID, 100000.0, 160000.0, 4000000.0, 3000000.0)
			requests = append(requests, embed...)
		}
	}

	if len(requests) == 0 {
		return nil
	}

	_, err = slidesSvc.Presentations.BatchUpdate(presentationID, &slides.BatchUpdatePresentationRequest{Requests: requests}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("batch update: %w", err)
	}
	return nil
}
