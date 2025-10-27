package charts

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/sheets/v4"
	"google.golang.org/api/slides/v1"
)

// Point represents a single labeled numeric value.
type Point struct {
	Label string
	Value float64
}

// DatasetSpec describes a small dataset suitable for a single chart.
type DatasetSpec struct {
	Title  string
	Unit   string
	Type   string // timeseries | category | comparison
	Points []Point
}

// CreateSheetsChart writes the dataset into the given spreadsheet's sheet (creating it if needed),
// clears prior data, wipes existing chart sheets, and creates a new chart. Returns: chartID, error.
func CreateSheetsChart(ctx context.Context, sheetsSvc *sheets.Service, spreadsheetID string, sheetTitle string, ds DatasetSpec) (int64, error) {
	if sheetsSvc == nil {
		return 0, fmt.Errorf("sheetsSvc is nil")
	}
	if strings.TrimSpace(spreadsheetID) == "" {
		return 0, fmt.Errorf("spreadsheetID is required")
	}
	if strings.TrimSpace(sheetTitle) == "" {
		sheetTitle = "Data"
	}
	if len(ds.Points) == 0 {
		return 0, fmt.Errorf("no points to chart")
	}

	// Ensure sheet exists, get its ID
	sheetID, err := ensureGridSheet(ctx, sheetsSvc, spreadsheetID, sheetTitle)
	if err != nil {
		return 0, err
	}

	// Clear previous values on the target sheet
	_, err = sheetsSvc.Spreadsheets.Values.Clear(spreadsheetID, sheetTitle+"!A:Z", &sheets.ClearValuesRequest{}).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("clear values: %w", err)
	}

	// Wipe previous chart sheets
	if err := deleteAllChartSheets(ctx, sheetsSvc, spreadsheetID); err != nil {
		return 0, err
	}

	// Prepare typed values then convert at the boundary
	headerValue := "Value"
	if ds.Unit != "" {
		headerValue = fmt.Sprintf("Value (%s)", ds.Unit)
	}
	labels := make([]string, 0, len(ds.Points))
	nums := make([]float64, 0, len(ds.Points))
	for _, p := range ds.Points {
		labels = append(labels, p.Label)
		nums = append(nums, p.Value)
	}
	values := makeCells(labels, headerValue, nums)
	vr := &sheets.ValueRange{Values: values}
	if _, err := sheetsSvc.Spreadsheets.Values.Update(spreadsheetID, sheetTitle+"!A1:B", vr).ValueInputOption("RAW").Context(ctx).Do(); err != nil {
		return 0, fmt.Errorf("write values: %w", err)
	}

	// Define chart type
	chartType := "COLUMN"
	switch ds.Type {
	case "timeseries":
		chartType = "LINE"
	case "category", "comparison":
		chartType = "COLUMN"
	}

	// Build chart spec using ranges (A2:A, B2:B)
	rowCount := int64(len(ds.Points) + 1) // including header
	domainRange := &sheets.GridRange{SheetId: sheetID, StartRowIndex: 1, EndRowIndex: rowCount, StartColumnIndex: 0, EndColumnIndex: 1}
	seriesRange := &sheets.GridRange{SheetId: sheetID, StartRowIndex: 1, EndRowIndex: rowCount, StartColumnIndex: 1, EndColumnIndex: 2}

	addChartReq := &sheets.AddChartRequest{
		Chart: &sheets.EmbeddedChart{
			Spec: &sheets.ChartSpec{
				Title: nonEmpty(ds.Title, "Chart"),
				BasicChart: &sheets.BasicChartSpec{
					ChartType:      chartType,
					LegendPosition: "BOTTOM_LEGEND",
					Domains: []*sheets.BasicChartDomain{
						{Domain: &sheets.ChartData{SourceRange: &sheets.ChartSourceRange{Sources: []*sheets.GridRange{domainRange}}}},
					},
					Series: []*sheets.BasicChartSeries{
						{Series: &sheets.ChartData{SourceRange: &sheets.ChartSourceRange{Sources: []*sheets.GridRange{seriesRange}}}, TargetAxis: "LEFT_AXIS"},
					},
				},
			},
			Position: &sheets.EmbeddedObjectPosition{NewSheet: true},
		},
	}

	breq := &sheets.BatchUpdateSpreadsheetRequest{Requests: []*sheets.Request{{AddChart: addChartReq}}}
	bresp, err := sheetsSvc.Spreadsheets.BatchUpdate(spreadsheetID, breq).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("batch update (add chart): %w", err)
	}
	if bresp == nil || len(bresp.Replies) == 0 || bresp.Replies[0].AddChart == nil || bresp.Replies[0].AddChart.Chart == nil {
		return 0, fmt.Errorf("missing add chart reply")
	}
	chartID := bresp.Replies[0].AddChart.Chart.ChartId

	return chartID, nil
}

// BuildEmbedRequests creates Slides requests to embed the given Sheets chart into a slide.
// Position and size use EMU units to match official examples.
func BuildEmbedRequests(spreadsheetID string, chartID int64, pageObjectID string, objectID string, xEMU, yEMU, widthEMU, heightEMU float64) []*slides.Request {
	if objectID == "" {
		objectID = "MyEmbeddedChart"
	}
	if widthEMU <= 0 {
		widthEMU = 4000000
	}
	if heightEMU <= 0 {
		heightEMU = 4000000
	}
	if xEMU < 0 {
		xEMU = 100000
	}
	if yEMU < 0 {
		yEMU = 100000
	}

	emuW := slides.Dimension{Magnitude: widthEMU, Unit: "EMU"}
	emuH := slides.Dimension{Magnitude: heightEMU, Unit: "EMU"}

	return []*slides.Request{
		{
			CreateSheetsChart: &slides.CreateSheetsChartRequest{
				ObjectId:      objectID,
				SpreadsheetId: spreadsheetID,
				ChartId:       chartID,
				LinkingMode:   "LINKED",
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: pageObjectID,
					Size: &slides.Size{
						Height: &emuH,
						Width:  &emuW,
					},
					Transform: &slides.AffineTransform{ScaleX: 1.0, ScaleY: 1.0, TranslateX: xEMU, TranslateY: yEMU, Unit: "EMU"},
				},
			},
		},
	}
}

func nonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// makeCells converts typed label/value slices into [][]interface{} expected by the Sheets API.
func makeCells(labels []string, header string, nums []float64) [][]interface{} {
	out := make([][]interface{}, 0, len(nums)+1)
	out = append(out, []interface{}{"Label", header}) //nolint
	for i := range nums {
		out = append(out, []interface{}{labels[i], nums[i]}) //nolint
	}
	return out
}

func ensureGridSheet(ctx context.Context, sheetsSvc *sheets.Service, spreadsheetID, sheetTitle string) (int64, error) {
	// Try to find existing sheet
	ss, err := sheetsSvc.Spreadsheets.Get(spreadsheetID).
		Fields("sheets(properties(sheetId,title,sheetType))").
		Context(ctx).
		Do()
	if err != nil {
		return 0, fmt.Errorf("get spreadsheet: %w", err)
	}
	for _, sh := range ss.Sheets {
		if sh != nil && sh.Properties != nil && sh.Properties.Title == sheetTitle {
			return sh.Properties.SheetId, nil
		}
	}

	// Create sheet
	bu := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{AddSheet: &sheets.AddSheetRequest{Properties: &sheets.SheetProperties{Title: sheetTitle}}},
		},
	}
	resp, err := sheetsSvc.Spreadsheets.BatchUpdate(spreadsheetID, bu).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("add sheet %q: %w", sheetTitle, err)
	}
	if resp == nil || len(resp.Replies) == 0 || resp.Replies[0].AddSheet == nil || resp.Replies[0].AddSheet.Properties == nil {
		return 0, fmt.Errorf("missing add sheet reply")
	}
	return resp.Replies[0].AddSheet.Properties.SheetId, nil
}

func deleteAllChartSheets(ctx context.Context, sheetsSvc *sheets.Service, spreadsheetID string) error {
	ss, err := sheetsSvc.Spreadsheets.Get(spreadsheetID).
		Fields("sheets(properties(sheetId,sheetType))").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("get spreadsheet (for chart wipe): %w", err)
	}
	var reqs []*sheets.Request
	for _, sh := range ss.Sheets {
		if sh == nil || sh.Properties == nil {
			continue
		}
		if strings.EqualFold(sh.Properties.SheetType, "CHART") {
			reqs = append(reqs, &sheets.Request{DeleteSheet: &sheets.DeleteSheetRequest{SheetId: sh.Properties.SheetId}})
		}
	}
	if len(reqs) == 0 {
		return nil
	}
	_, err = sheetsSvc.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{Requests: reqs}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("delete chart sheets: %w", err)
	}
	return nil
}
