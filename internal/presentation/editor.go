package presentation

import (
	"context"
	"fmt"

	"gogemini-practices/internal/formatting"

	"google.golang.org/api/slides/v1"
)

type Topic struct {
	Title   string
	Summary string
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
		targetSlideIDs = append(targetSlideIDs, pres.Slides[i].ObjectId)
	}
	for i := existing; i < need; i++ {
		slideID := fmt.Sprintf("auto_slide_%d", i)
		targetSlideIDs = append(targetSlideIDs, slideID)
		requests = append(requests, &slides.Request{CreateSlide: &slides.CreateSlideRequest{
			ObjectId:             slideID,
			SlideLayoutReference: &slides.LayoutReference{PredefinedLayout: "BLANK"},
		}})
	}

	for i := range need {
		slideID := targetSlideIDs[i]
		titleID := fmt.Sprintf("auto_title_%d", i)
		bodyID := fmt.Sprintf("auto_body_%d", i)

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
