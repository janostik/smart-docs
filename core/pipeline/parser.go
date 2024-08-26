package pipeline

import (
	"cmp"
	"fmt"
	"slices"
	"smart-docs/core/models"
)

const minOverlap = 0.4

type segment struct {
	*models.Prediction
	content string
	words   []models.WordData
}

func (s *segment) realign() {
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
	segments := make([]segment, len(*predictions))

	for i := range *predictions {
		segments[i] = segment{
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
	for _, s := range segments {
		switch s.Label {
		case "table":
			// TODO: Parse table:
			html += "\n<table>TODO TABLE<table>\n"
			break
		case "paragraph":
			html += fmt.Sprintf("<p>%s</p>", s.content)
		case "header":
			html += fmt.Sprintf("<h5>%s</h5>", s.content)
		default:
			html += fmt.Sprintf("<span>%s</span>", s.content)
		}
	}
	return html
}

func lookupBestSegment(word models.WordData, segments *[]segment) *segment {

	//	find first smallest segment that overlaps with word polygon
	//	we pick smallest, since bigger segments have bigger chance of incorrectly overlapping neighbouring segments
	var overlappingSegments []*segment
	for i := range *segments {
		s := &(*segments)[i]
		if intersection(word, s) > minOverlap {
			overlappingSegments = append(overlappingSegments, s)
		}
	}
	areaCmp := func(a, b *segment) int {
		return cmp.Compare(area(a.Rect), area(b.Rect))
	}
	slices.SortFunc(overlappingSegments, areaCmp)

	if len(overlappingSegments) > 0 {
		return overlappingSegments[0]
	} else {
		return nil
	}
}

func intersection(word models.WordData, s *segment) float32 {
	overlap := overlapArea(word, s)
	return overlap / area(word.Rect)
}

func overlapArea(w models.WordData, p *segment) float32 {
	xOverlap := max(0.0, min(w.X1, p.X1)-max(w.X0, p.X0))
	yOverlap := max(0.0, min(w.Y1, p.Y1)-max(w.Y0, p.Y0))
	return xOverlap * yOverlap
}

func area(r models.Rect) float32 {
	return (r.X1 - r.X0) * (r.Y1 - r.Y0)
}
