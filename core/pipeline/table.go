package pipeline

import (
	"cmp"
	"slices"
	"smart-docs/core/models"
)

type Cell struct {
	*models.Prediction
	content     string
	OffsetStart float32
	Colspan     int
	Rowspan     int
}

const maxOverlap = 0.8

func (s *Segment) ParseTable() [][]Cell {

	var cells []Cell
	for _, p := range s.Prediction.Table {
		if p.Y1-p.Y0 < 16 {
			// cleanup due to bad segmentation to handle small cells
			// maybe we should add boundary to tables.
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
			if otherIndex < index || contains(overlappingCells, otherIndex) {
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

	yCmp := func(a, b Cell) int {
		return cmp.Compare(a.Y0, b.Y0)
	}
	slices.SortFunc(cells, yCmp)

	// Make a copy of the cells slice for processing
	cellsToProcess := make([]Cell, len(cells))
	copy(cellsToProcess, cells)

	var table [][]Cell
	topLeft := findTopLeftCell(cellsToProcess)
	deleteFromSlice(&cellsToProcess, topLeft)
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
		deleteFromSlice(&cellsToProcess, topLeft)
	}

	if len(currentRow) > 0 {
		table = append(table, currentRow)
	}
	// END: BUILDING OF TABLE

	// START: HANDLE ROW SPANS
	minCols := 0
	maxCols := 999
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
			cell.OffsetStart = offset
			offset = offset + cell.Width()
		}
	}

	xGrid := make([]float32, maxCols+1)
	yGrid := make([]float32, len(table))

	for r := range len(table) {
		row := table[r]
		if len(row) == maxCols {
			for c := range maxCols {
				cell := row[c]
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
		var avgY float32 = 0
		for c := range minCols {
			avgY = avgY + row[c].Y1
		}
		avgY = avgY / float32(minCols)
		yGrid[r] = avgY
	}

	for r, _ := range table {
		for c, _ := range table[r] {
			cell := table[r][c]
			cell.X0 = nearest(cell.X0, xGrid)
			cell.X1 = nearest(cell.X1, xGrid)
			cell.Y0 = nearest(cell.Y0, yGrid)
			cell.Y1 = nearest(cell.Y1, yGrid)
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

			cell.Colspan = colspan
			cell.Rowspan = rowspan
		}
	}
	// END: HANDLE ROW SPANS

	s.Prediction.Table = []models.Prediction{}
	//s.Prediction.Table = make([]models.Prediction, len(cells))
	for r, _ := range table {
		row := table[r]
		for c, _ := range row {
			cell := row[c]
			s.Prediction.Table = append(s.Prediction.Table, *cell.Prediction)
		}
	}

	return table
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

func deleteFromSlice(input *[]Cell, toDelete Cell) {

	for i, cell := range *input {
		if toDelete.X0 == cell.X0 && toDelete.X1 == cell.X1 && toDelete.Y0 == cell.Y0 && toDelete.Y1 == cell.Y1 {
			*input = slices.Delete(*input, i, i)
			return
		}
	}

}

func findCellsToRight(topLeft Cell, row []Cell) []Cell {
	var result []Cell
	for c, _ := range row {
		cell := row[c]
		isWithinY := topLeft.Y0 < cell.CenterY() && cell.CenterY() < topLeft.Y1
		if cell.CenterX() > topLeft.X1 && isWithinY {
			result = append(result, cell)
		}
	}
	return result
}

func findTopLeftCell(row []Cell) Cell {
	return row[0]
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
