package pipeline

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"os"
	"smart-docs/core/db"
	"smart-docs/core/mistral"
	"smart-docs/core/models"
	"smart-docs/core/pipeline/markdown"

	"github.com/llgcode/draw2d/draw2dimg"
	"golang.org/x/image/colornames"
)

func ProcessPdf(docId int64, shouldRunOcr bool, mode string) {
	var pageCount int
	var words [][]models.WordData
	var ocrWords [][]models.WordData
	var markdownPages []string
	var err error

	if mode == "mistral" {
		client, err := mistral.NewClient()
		if err != nil {
			log.Printf("Error creating Mistral client: \n%+v", err)
			return
		}

		filePath := fmt.Sprintf("./data/files/%d.pdf", docId)
		fileId, err := client.UploadFile(filePath)
		if err != nil {
			log.Printf("Error uploading file to Mistral: \n%+v", err)
			return
		}

		err = db.UpdateMistralFileId(docId, fileId)
		if err != nil {
			log.Printf("Error updating mistral_file_id: \n%+v", err)
			return
		}

		markdownPages, err = client.ParseFile(fileId)
		if err != nil {
			log.Printf("Error parsing file with Mistral: \n%+v", err)
			return
		}

		pageCount = len(markdownPages)
		words = make([][]models.WordData, pageCount)
		ocrWords = make([][]models.WordData, pageCount)
	}

	pageCount, words, err = storeImagesAndExtractPages(docId)
	if err != nil {
		log.Printf("Error while extracting images: \n%+v", err)
		return
	}

	err = db.UpdatePageCount(docId, pageCount)
	if err != nil {
		log.Printf("Error while calling core.db: \n%+v", err)
		return
	}

	if shouldRunOcr && mode == "manual" {
		ocrWords, err = DetectAsyncDocumentURI(docId, pageCount)
		if err != nil {
			log.Printf("Error while running ocr: \n%+v", err)
			return
		}
	}

	var pages = make([]models.Page, pageCount)

	for p := range pages {
		log.Printf("Annotating page: %d", p)

		page := &pages[p]
		page.DocumentId = docId
		page.PageNum = p
		page.Status = "PREDICTION"

		err := GetPageDimensions(page)
		if err != nil {
			log.Printf("Error getting image dimensions: \n%+v", err)
			return
		}

		page.PdfText = words[p]
		if ocrWords != nil {
			serialisedOcr, err := json.Marshal(ocrWords[p])
			if err != nil {
				log.Printf("Error serialising ocr data: \n%+v", err)
				return
			}
			page.OcrText = string(serialisedOcr)
		}

		var predictions []models.Prediction
		if mode == "manual" {
			predictions, err = RunDetectionOnPage(docId, p)
			DrawBoundingBoxes(docId, p, &predictions, "original")
			if err != nil {
				log.Printf("Error detecting segments: \n%+v", err)
				return
			}
			if shouldRunOcr {
				page.Html = ParseHtmlAndAdjustDetection(&ocrWords[p], &predictions, docId, p)
			} else {
				page.Html = ParseHtmlAndAdjustDetection(&words[p], &predictions, docId, p)
			}
			DrawBoundingBoxes(docId, p, &predictions, "prediction")
		} else if mode == "mistral" {
			predictions = []models.Prediction{}
			page.Md = markdownPages[p]
			html, err := markdown.ConvertMarkdownToHTML(page.Md)
			if err != nil {
				log.Printf("Error converting to html: \n%+v", err)
				return
			}
			page.Html = html
		}
		page.Predictions = predictions
	}

	err = db.StorePages(&pages)
	if err != nil {
		log.Printf("Error while calling core.db: \n%+v", err)
		return
	}

	err = db.UpdateDocumentStatus(docId, "DONE")
	if err != nil {
		log.Printf("Error while calling core.db: \n%+v", err)
		return
	}
}

func DrawBoundingBoxes(docId int64, page int, predictions *[]models.Prediction, suffix string) {
	imgFile, err := os.Open(fmt.Sprintf("./data/images/%d/%d.jpg", docId, page))
	if err != nil {
		log.Panicf("Cannot open image: \n%+v", err)
	}
	defer imgFile.Close()
	img, err := jpeg.Decode(imgFile)
	if err != nil {
		log.Panicf("Cannot decode image: \n%+v", err)
	}
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)
	gc := draw2dimg.NewGraphicContext(rgba)
	gc.SetLineWidth(1)
	for _, prediction := range *predictions {
		switch prediction.Label {
		case "table":
			gc.SetStrokeColor(colornames.Gray)
		case "paragraph":
			gc.SetStrokeColor(colornames.Blue)
		case "header":
			gc.SetStrokeColor(colornames.Red)
		case "illustration":
			gc.SetStrokeColor(colornames.Yellow)
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
	outFile, err := os.Create(fmt.Sprintf("./data/images/%d/%d.%s.jpg", docId, page, suffix))
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

func RetryAnnotations(docId int64) {
	pagesToProcess, err := db.GetNonValidatedPages(docId)
	if err != nil {
		return
	}
	err = db.UpdateDocumentStatus(docId, "PROCESSING")
	if err != nil {
		log.Println(fmt.Sprintf("Failed to update document status: \n%+v", err))
		return
	}
	go reprocessPages(docId, pagesToProcess)
}

func reprocessPages(docId int64, pages []int) {
	for _, p := range pages {
		predictions, err := RunDetectionOnPage(docId, p)
		DrawBoundingBoxes(docId, p, &predictions, "original")
		if err != nil {
			log.Println(fmt.Sprintf("Error detecting segments: \n%+v", err))
			return
		}
		words, err := db.GetPdfPageText(docId, p)
		if err != nil {
			log.Println(fmt.Sprintf("Could not fetch pdf text: \n%+v", err))
			return
		}
		html := ParseHtmlAndAdjustDetection(&words, &predictions, docId, p)
		DrawBoundingBoxes(docId, p, &predictions, "prediction")
		err = db.UpdatePredictionsAndText(docId, p, &predictions, &html)
		if err != nil {
			log.Println(fmt.Sprintf("Error updating document predictions and text: \n%+v", err))
			return
		}
	}

	err := db.UpdateDocumentStatus(docId, "DONE")
	if err != nil {
		log.Println(fmt.Sprintf("Failed to update document status: \n%+v", err))
		return
	}
}
