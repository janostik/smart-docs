package pipeline

import (
	"smart-docs/core/models"
)

type Cell struct {
	*models.Prediction
	content string
}

const maxOverlap = 0.8

func (s *Segment) ParseTable() [][]Cell {

	var cells []models.Prediction
	for _, p := range s.Prediction.Table {
		if p.Y1-p.Y0 < 16 {
			// cleanup due to bad segmentation to handle small cells
			// maybe we should add boundary to tables.
			continue
		}
		cells = append(cells, p)
	}

	var overlappingCells []int
	for index, _ := range cells {
		var cell = cells[index]
		for otherIndex, _ := range cells {
			var otherCell = cells[otherIndex]
			if otherIndex < index || contains(overlappingCells, otherIndex) {
				continue
			}
			var overlap = Intersection(cell.Rect, otherCell.Rect)
			if overlap > maxOverlap {
				overlappingCells = append(overlappingCells, otherIndex)
			} else if overlap > 0 {
				var currentSmaller = Area(cell.Rect) <= Area(otherCell.Rect)
				var cellToKeep *models.Prediction
				var cellToAdjust *models.Prediction
				if currentSmaller {
					cellToKeep = &cell
					cellToAdjust = &otherCell
				} else {
					cellToKeep = &otherCell
					cellToAdjust = &cell
				}
				var diffs = [4]float32{
					absDiff(cellToKeep.X0, cellToAdjust.X1),
					absDiff(cellToKeep.Y0, cellToAdjust.Y1),
					absDiff(cellToKeep.X1, cellToAdjust.X0),
					absDiff(cellToKeep.Y1, cellToAdjust.Y0),
				}
				var sideToAdjust = indexOfSmallest(diffs)
				if sideToAdjust == 0 { // left side
					cellToAdjust.X1 = cellToKeep.X0
				} else if sideToAdjust == 1 { // top side
					cellToAdjust.Y1 = cellToKeep.Y0
				} else if sideToAdjust == 2 { // right side
					cellToAdjust.X0 = cellToKeep.X1
				} else if sideToAdjust == 3 { // bottom side
					cellToAdjust.Y0 = cellToKeep.Y1
				}
			}
		}
	}

	s.Prediction.Table = make([]models.Prediction, len(cells))
	for i, cell := range cells {
		s.Prediction.Table[i] = cell
	}
	//s.Prediction.Table = cells

	//result := make([][]Cell, 0)
	//if len(*predictions) == 0 {
	//	return cells
	//}

	return make([][]Cell, 0)
}

func indexOfSmallest(diffs [4]float32) int {
	var index = -1
	var smallestValue float32 = 9999.0
	for d, diff := range diffs {
		if diff < smallestValue {
			index = d
			smallestValue = diff
		}
	}
	return index
}

func contains(array []int, value int) bool {
	for _, item := range array {
		if item == value {
			return true
		}
	}
	return false
}

func absDiff(a, b float32) float32 {
	if a > b {
		return a - b
	} else {
		return b - a
	}
}
