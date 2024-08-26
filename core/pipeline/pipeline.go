package pipeline

import (
	"fmt"
	"github.com/llgcode/draw2d/draw2dimg"
	"golang.org/x/image/colornames"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"os"
	"smart-docs/core/db"
	"smart-docs/core/models"
)

func ProcessPdf(docId int64) {
	pageCount, words, err := storeImagesAndExtractPages(docId)
	if err != nil {
		log.Println(fmt.Sprintf("Error while extracting images: \n%+v", err))
		return
	}

	err = db.UpdatePageCount(docId, pageCount)
	if err != nil {
		log.Println(fmt.Sprintf("Error while calling core.db: \n%+v", err))
		return
	}

	var pages = make([]models.Page, pageCount)

	for p := range pages {
		page := &pages[p]
		page.DocumentId = docId
		page.PageNum = p
		page.Status = "PREDICTION"

		page.PdfText = words[p]
		// TODO: Update pages with OCR
		page.OcrText = ""
		predictions, err := RunDetectionOnPage(docId, p)
		drawBoundingBoxes(docId, p, &predictions)
		if err != nil {
			log.Println(fmt.Sprintf("Error detecting segments: \n%+v", err))
			return
		}
		page.Html = ParseHtmlAndAdjustDetection(&words[p], &predictions)
		page.Predictions = predictions
	}

	err = db.StorePages(&pages)
	if err != nil {
		log.Println(fmt.Sprintf("Error while calling core.db: \n%+v", err))
		return
	}

	err = db.UpdateDocumentStatus(docId, "DONE")
	if err != nil {
		log.Println(fmt.Sprintf("Error while calling core.db: \n%+v", err))
		return
	}
}

func drawBoundingBoxes(docId int64, page int, predictions *[]models.Prediction) {
	imgFile, err := os.Open(fmt.Sprintf("cmd/web/assets/images/%d/%d.jpg", docId, page))
	if err != nil {
		log.Panicln(fmt.Sprintf("Cannot open image: \n%+v", err))
	}
	defer imgFile.Close()
	img, err := jpeg.Decode(imgFile)
	if err != nil {
		log.Panicln(fmt.Sprintf("Cannot decode image: \n%+v", err))
	}
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)
	gc := draw2dimg.NewGraphicContext(rgba)
	gc.SetLineWidth(4)
	for _, prediction := range *predictions {
		switch prediction.Label {
		case "table":
			gc.SetStrokeColor(colornames.Gray)
		case "paragraph":
			gc.SetStrokeColor(colornames.Blue)
		case "header":
			gc.SetStrokeColor(colornames.Red)
		default:
			gc.SetStrokeColor(colornames.Green)
		}
		drawBox(gc, prediction.X0, prediction.Y0, prediction.X1, prediction.Y1)
		if prediction.Label == "table" {
			for _, cellPrediction := range prediction.Table {
				switch cellPrediction.Label {
				case "cell":
					gc.SetStrokeColor(colornames.Blue)
				case "header":
					gc.SetStrokeColor(colornames.Red)
				}
				drawBox(gc,
					cellPrediction.X0+prediction.X0,
					cellPrediction.Y0+prediction.Y0,
					cellPrediction.X1+prediction.X0,
					cellPrediction.Y1+prediction.Y0,
				)
			}
		}
	}
	outFile, err := os.Create(fmt.Sprintf("cmd/web/assets/images/%d/%d.prediction.jpg", docId, page))
	if err != nil {
		log.Panicln(fmt.Sprintf("Cannot open image: \n%+v", err))
	}
	defer outFile.Close()

	jpeg.Encode(outFile, rgba, nil)
}

func drawBox(gc *draw2dimg.GraphicContext, x0 float32, y0 float32, x1 float32, y1 float32) {
	gc.BeginPath()
	gc.MoveTo(float64(x0), float64(y0))
	gc.LineTo(float64(x1), float64(y0))
	gc.LineTo(float64(x1), float64(y1))
	gc.LineTo(float64(x0), float64(y1))
	gc.Close()
	gc.Stroke()
}
