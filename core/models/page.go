package models

import "html/template"

type Rect struct {
	X0 float32 `json:"x0"`
	X1 float32 `json:"x1"`
	Y0 float32 `json:"y0"`
	Y1 float32 `json:"y1"`
}

func (r Rect) Width() float32 {
	return r.X1 - r.X0
}

func (r Rect) CenterX() float32 {
	return (r.X0 + r.X1) / 2
}

func (r Rect) CenterY() float32 {
	return (r.Y0 + r.Y1) / 2
}

type WordData struct {
	Rect
	Text string
}

type Prediction struct {
	Rect
	Score float32      `json:"score"`
	Label string       `json:"label"`
	Table []Prediction `json:"table"`
}

type Page struct {
	Id          int64
	DocumentId  int64
	PageNum     int
	Orientation int
	PdfText     []WordData
	OcrText     string
	Status      string
	Predictions []Prediction
	Html        string
}

type PageView struct {
	Id              int64
	DocumentName    string
	DocumentId      int64
	Status          string
	PageNum         int
	NextPage        int
	PreviousPage    int
	HasNextPage     bool
	HasPreviousPage bool
	Html            template.HTML
}

// TODO: Only allow 2 states from doc view and 3 states from training view
