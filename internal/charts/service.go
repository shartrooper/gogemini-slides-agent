package charts

import (
	"context"
	"fmt"

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

// CreateSheetsChart creates a new spreadsheet with the dataset, adds a chart, and returns IDs for embedding.
// Returns: spreadsheetID, chartID, error.
func CreateSheetsChart(ctx context.Context, sheetsSvc *sheets.Service, ds DatasetSpec) (string, int64, error) {
	if sheetsSvc == nil {
		return "", 0, fmt.Errorf("sheetsSvc is nil")
	}
	if len(ds.Points) == 0 {
		return "", 0, fmt.Errorf("no points to chart")
	}

	// Create spreadsheet with a single data sheet named "Data"
	spreadsheet, err := sheetsSvc.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{Title: nonEmpty(ds.Title, "Dataset")},
		Sheets: []*sheets.Sheet{
			{Properties: &sheets.SheetProperties{Title: "Data"}},
		},
	}).Context(ctx).Do()
	if err != nil {
		return "", 0, fmt.Errorf("create spreadsheet: %w", err)
	}
	spreadsheetID := spreadsheet.SpreadsheetId
	if spreadsheetID == "" || len(spreadsheet.Sheets) == 0 || spreadsheet.Sheets[0].Properties == nil {
		return "", 0, fmt.Errorf("invalid spreadsheet create response")
	}
	sheetID := spreadsheet.Sheets[0].Properties.SheetId
	if sheetID == 0 {
		return "", 0, fmt.Errorf("missing sheet id")
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
	if _, err := sheetsSvc.Spreadsheets.Values.Update(spreadsheetID, "Data!A1:B", vr).ValueInputOption("RAW").Context(ctx).Do(); err != nil {
		return "", 0, fmt.Errorf("write values: %w", err)
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
		return "", 0, fmt.Errorf("batch update (add chart): %w", err)
	}
	if bresp == nil || len(bresp.Replies) == 0 || bresp.Replies[0].AddChart == nil || bresp.Replies[0].AddChart.Chart == nil {
		return "", 0, fmt.Errorf("missing add chart reply")
	}
	chartID := bresp.Replies[0].AddChart.Chart.ChartId
	return spreadsheetID, chartID, nil
}

// BuildEmbedRequests creates Slides requests to embed the given Sheets chart into a slide.
// Position and size are in points (PT). If width/height are zero, sensible defaults are used.
func BuildEmbedRequests(spreadsheetID string, chartID int64, pageObjectID string, xPT, yPT, widthPT, heightPT float64) []*slides.Request {
	if widthPT <= 0 {
		widthPT = 500
	}
	if heightPT <= 0 {
		heightPT = 300
	}
	if xPT < 0 {
		xPT = 60
	}
	if yPT < 0 {
		yPT = 160
	}

	return []*slides.Request{
		{
			CreateSheetsChart: &slides.CreateSheetsChartRequest{
				SpreadsheetId: spreadsheetID,
				ChartId:       chartID,
				LinkingMode:   "LINKED",
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: pageObjectID,
					Size: &slides.Size{
						Width:  &slides.Dimension{Magnitude: widthPT, Unit: "PT"},
						Height: &slides.Dimension{Magnitude: heightPT, Unit: "PT"},
					},
					Transform: &slides.AffineTransform{ScaleX: 1, ScaleY: 1, TranslateX: xPT, TranslateY: yPT, Unit: "PT"},
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
