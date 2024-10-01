package pipeline

import (
	"cmp"
	"fmt"
	"slices"
	"smart-docs/core/models"
	"strings"
)

const minOverlap = 0.4

type Segment struct {
	*models.Prediction
	content string
	words   []models.WordData
}

func (s *Segment) realign() {
	if len(s.words) == 0 {
		return
	}
	if s.Prediction.Label == "table" {
		return
	}
	var x0 float32 = 99999.0
	var y0 float32 = 99999.0
	var x1 float32 = -99999.0
	var y1 float32 = -99999.0

	for _, word := range s.words {
		if word.X0 < x0 {
			x0 = word.X0
		}
		if word.Y0 < y0 {
			y0 = word.Y0
		}
		if word.X1 > x1 {
			x1 = word.X1
		}
		if word.Y1 > y1 {
			y1 = word.Y1
		}
	}
	s.Prediction.X0 = x0
	s.Prediction.Y0 = y0
	s.Prediction.X1 = x1
	s.Prediction.Y1 = y1
}

func ParseHtmlAndAdjustDetection(words *[]models.WordData, predictions *[]models.Prediction) string {
	segments := make([]Segment, len(*predictions))

	for i := range *predictions {
		segments[i] = Segment{
			content:    "",
			Prediction: &(*predictions)[i],
			words:      make([]models.WordData, 0),
		}
	}

	for _, word := range *words {
		segment := lookupBestSegment(word, &segments)
		if segment != nil {
			segment.content = segment.content + " " + word.Text
			segment.words = append(segment.words, word)
		}
	}

	for i := range segments {
		s := segments[i]
		s.realign()
	}

	// TODO: Extract different renderers
	var html = ""
	for s, _ := range segments {
		segment := segments[s]
		switch segment.Label {
		case "table":
			table := segment.ParseTable()
			var b strings.Builder
			b.WriteString("<table>")
			for _, row := range table {
				b.WriteString("<tr>")
				for _, cell := range row {
					b.WriteString(fmt.Sprintf("<td colspan=\"%d\" rowspan=\"%d\">", cell.Colspan, cell.Rowspan))
					b.WriteString(cell.content)
					b.WriteString("</td>")
				}
				b.WriteString("</tr>")
			}
			b.WriteString("</table>")
			html += b.String()
			break
		case "paragraph":
			html += fmt.Sprintf("<p>%s</p>", segment.content)
		case "header":
			html += fmt.Sprintf("<h5>%s</h5>", segment.content)
		default:
			html += fmt.Sprintf("<span>%s</span>", segment.content)
		}
	}
	return html
}

func lookupBestSegment(word models.WordData, segments *[]Segment) *Segment {

	//	find first smallest segment that overlaps with word polygon
	//	we pick smallest, since bigger segments have bigger chance of incorrectly overlapping neighbouring segments
	var overlappingSegments []*Segment
	for i := range *segments {
		s := &(*segments)[i]
		if Intersection(word.Rect, s.Rect) > minOverlap {
			overlappingSegments = append(overlappingSegments, s)
		}
	}
	areaCmp := func(a, b *Segment) int {
		return cmp.Compare(Area(a.Rect), Area(b.Rect))
	}
	slices.SortFunc(overlappingSegments, areaCmp)

	if len(overlappingSegments) > 0 {
		return overlappingSegments[0]
	} else {
		return nil
	}
}

func Intersection(word models.Rect, s models.Rect) float32 {
	overlap := overlapArea(word, s)
	return overlap / Area(word)
}

func overlapArea(w models.Rect, p models.Rect) float32 {
	xOverlap := max(0.0, min(w.X1, p.X1)-max(w.X0, p.X0))
	yOverlap := max(0.0, min(w.Y1, p.Y1)-max(w.Y0, p.Y0))
	return xOverlap * yOverlap
}

func Area(r models.Rect) float32 {
	return (r.X1 - r.X0) * (r.Y1 - r.Y0)
}
