package pipeline

import (
	"cmp"
	"slices"
	"smart-docs/core/models"
)

type Cell struct {
	*models.Prediction
	content     string
	words       []models.WordData
	OffsetStart float32
	Colspan     int
	Rowspan     int
}

const maxOverlap = 0.8

func (s *Segment) ParseTable() [][]Cell {

	var cells []Cell
	for _, p := range s.Prediction.Table {
		if p.X0 > p.X1 || p.Y0 > p.Y1 || p.X0 < 0 || p.Y0 < 0 {
			// Skip invalid cells
			continue
		}
		cells = append(cells, Cell{
			Prediction: &p,
		})
	}

	var overlappingCells []int
	for index := range cells {
		cell := cells[index]
		for otherIndex := range cells {
			otherCell := cells[otherIndex]
			if otherIndex < index || contains(overlappingCells, index) {
				continue
			}
			overlap := Intersection(cell.Rect, otherCell.Rect)
			if overlap > maxOverlap {
				overlappingCells = append(overlappingCells, otherIndex)
			} else if overlap > 0 {
				currentSmaller := Area(cell.Rect) <= Area(otherCell.Rect)
				var cellToKeep Cell
				var cellToAdjust Cell
				if currentSmaller {
					cellToKeep = cell
					cellToAdjust = otherCell
				} else {
					cellToKeep = otherCell
					cellToAdjust = cell
				}
				diffs := [4]float32{
					absDiff(cellToKeep.X0, cellToAdjust.X1),
					absDiff(cellToKeep.Y0, cellToAdjust.Y1),
					absDiff(cellToKeep.X1, cellToAdjust.X0),
					absDiff(cellToKeep.Y1, cellToAdjust.Y0),
				}
				sideToAdjust := indexOfSmallest(diffs)
				switch sideToAdjust {
				case 0: // left side
					cellToAdjust.X1 = cellToKeep.X0
				case 1: // top side
					cellToAdjust.Y1 = cellToKeep.Y0
				case 2: // right side
					cellToAdjust.X0 = cellToKeep.X1
				case 3: // bottom side
					cellToAdjust.Y0 = cellToKeep.Y1
				}
			}
		}
	}

	// Assign content
	for _, word := range s.words {
		segment := lookupBestCell(word, &cells, s.X0, s.Y0)
		if segment != nil {
			segment.content = segment.content + " " + word.Text
			segment.words = append(segment.words, word)
		}
	}

	yCmp := func(a, b Cell) int {
		return cmp.Compare(a.Y0, b.Y0)
	}
	slices.SortFunc(cells, yCmp)

	// Make a copy of the cells slice for processing
	cellsToProcess := make([]Cell, len(cells))
	copy(cellsToProcess, cells)

	var table [][]Cell
	topLeft := findTopLeftCell(cellsToProcess)
	cellsToProcess = deleteFromSlice(cellsToProcess, topLeft)
	currentRow := []Cell{topLeft}

	// START: BUILDING OF TABLE
	for len(cellsToProcess) > 0 {
		cellsToRight := findCellsToRight(topLeft, cellsToProcess)
		if len(cellsToRight) > 0 {
			topLeft = findTopLeftCell(cellsToRight)
		} else {
			table = append(table, currentRow)
			topLeft = findTopLeftCell(cellsToProcess)
			currentRow = []Cell{}
		}
		currentRow = append(currentRow, topLeft)
		cellsToProcess = deleteFromSlice(cellsToProcess, topLeft)
	}

	if len(currentRow) > 0 {
		table = append(table, currentRow)
	}
	// END: BUILDING OF TABLE

	// START: HANDLE ROW SPANS
	minCols := 999
	maxCols := 0
	for r, _ := range table {
		row := table[r]
		if len(row) > maxCols {
			maxCols = len(row)
		}
		if len(row) < minCols {
			minCols = len(row)
		}
		offset := row[0].X0
		for c, _ := range row {
			cell := row[c]
			table[r][c].OffsetStart = offset
			cell.OffsetStart = offset
			offset = offset + cell.Width()
		}
	}

	xGrid := make([]float32, maxCols+1)
	yGrid := make([]float32, len(table)+1)

	for r := range len(table) {
		row := table[r]
		if len(row) == maxCols {
			for c := range maxCols {
				cell := row[c]
				// TODO: This is a bug. The row with most cells doesn't have all columns
				// To replicate. http://localhost:8080/annotate/29/301
				if xGrid[c] == 0 {
					xGrid[c] = cell.OffsetStart
				} else {
					xGrid[c] = (xGrid[c] + cell.OffsetStart) / 2
				}
				// We are at the last cell of the row. Let's add right border to the grid
				if c == (maxCols - 1) {
					if xGrid[maxCols] == 0 {
						xGrid[maxCols] = cell.OffsetStart + cell.Width()
					} else {
						xGrid[maxCols] = (xGrid[maxCols] + (cell.OffsetStart + cell.Width())) / 2
					}
				}
			}
		}
		var avgY0 float32 = 0
		for c := range minCols {
			avgY0 = avgY0 + row[c].Y0
		}
		avgY0 = avgY0 / float32(minCols)
		yGrid[r] = avgY0
		// Ensure we can snap to bottom border
		if r == len(table)-1 {
			var avgY1 float32 = 0
			for c := range minCols {
				avgY1 = avgY1 + row[c].Y1
			}
			avgY1 = avgY1 / float32(minCols)
			yGrid[r+1] = avgY1
		}
	}

	for r, _ := range table {
		for c, _ := range table[r] {
			cell := table[r][c]
			table[r][c].X0 = nearest(cell.X0, xGrid)
			table[r][c].X1 = nearest(cell.X1, xGrid)
			table[r][c].Y0 = nearest(cell.Y0, yGrid)
			table[r][c].Y1 = nearest(cell.Y1, yGrid)
		}
	}

	for r, _ := range table {
		row := table[r]
		for c, _ := range row {
			cell := row[c]
			colspan := 1
			if len(row) < maxCols {
				for _, x := range xGrid {
					if cell.X0 < x && x < cell.X1 {
						colspan++
					}
				}
			}
			rowspan := 1
			for _, y := range yGrid {
				if cell.Y0 < y && y < cell.Y1 {
					rowspan++
				}
			}

			table[r][c].Colspan = colspan
			table[r][c].Rowspan = rowspan
		}
	}
	// END: HANDLE ROW SPANS

	s.Prediction.Table = []models.Prediction{}
	//s.Prediction.Table = make([]models.Prediction, len(cells))
	for r, _ := range table {
		row := table[r]
		for c, _ := range row {
			cell := row[c]
			if isValidCell(cell, r, c, table) {
				s.Prediction.Table = append(s.Prediction.Table, *cell.Prediction)
			}
		}
	}

	return table
}

func isValidCell(cell Cell, testedRow int, testedCol int, table [][]Cell) bool {
	for r, _ := range table {
		row := table[r]
		for c, _ := range row {
			other := row[c]
			if r == testedRow && c == testedCol {
				continue
			}
			overlap := Intersection(cell.Rect, other.Rect)
			if overlap > 0.1 {
				return Area(cell.Rect) < Area(other.Rect)
			}
		}
	}
	return true
}

func nearest(item float32, grid []float32) float32 {
	var nearestItem float32
	var nearestDist float32 = 9999
	for _, candidate := range grid {
		dist := absDiff(candidate, item)
		if dist < nearestDist {
			nearestDist = dist
			nearestItem = candidate
		}
	}
	return nearestItem
}

func deleteFromSlice(input []Cell, toDelete Cell) []Cell {

	for i, cell := range input {
		if toDelete.X0 == cell.X0 && toDelete.X1 == cell.X1 && toDelete.Y0 == cell.Y0 && toDelete.Y1 == cell.Y1 {
			return slices.Delete(input, i, i+1)
		}
	}

	return input
}

func findCellsToRight(topLeft Cell, row []Cell) []Cell {
	var result []Cell
	for c, _ := range row {
		cell := row[c]
		isWithinY := false
		if cell.Height() > topLeft.Height() {
			isWithinY = cell.Y0 < topLeft.CenterY() && topLeft.CenterY() < cell.Y1
		} else {
			isWithinY = topLeft.Y0 < cell.CenterY() && cell.CenterY() < topLeft.Y1
		}
		if cell.CenterX() > topLeft.X1 && isWithinY {
			result = append(result, cell)
		}
	}
	return result
}

func findTopLeftCell(row []Cell) Cell {
	var minScore float32 = 99999
	var topLeftCell Cell
	var m float32 = 0.05 // slope. See this for more explanation: https://math.stackexchange.com/questions/2912005/get-the-top-left-most-point-from-random-points
	for c, _ := range row {
		cell := row[c]
		score := cell.Y0 + m*cell.X0
		if score < minScore {
			minScore = score
			topLeftCell = row[c]
		}
	}
	return topLeftCell
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

// TODO: Refactor to use shared function with parser
func lookupBestCell(word models.WordData, cells *[]Cell, offsetX float32, offsetY float32) *Cell {

	//	find first smallest segment that overlaps with word polygon
	//	we pick smallest, since bigger segments have bigger chance of incorrectly overlapping neighbouring segments
	var overlappingSegments []*Cell
	for i := range *cells {
		c := &(*cells)[i]
		//c.Rect
		if Intersection(word.Rect, models.Rect{X0: offsetX + c.X0, Y0: offsetY + c.Y0, X1: offsetX + c.X1, Y1: offsetY + c.Y1}) > minOverlap {
			overlappingSegments = append(overlappingSegments, c)
		}
	}
	areaCmp := func(a, b *Cell) int {
		return cmp.Compare(Area(a.Rect), Area(b.Rect))
	}
	slices.SortFunc(overlappingSegments, areaCmp)

	if len(overlappingSegments) > 0 {
		return overlappingSegments[0]
	} else {
		return nil
	}
}
