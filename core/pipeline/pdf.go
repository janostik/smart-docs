package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/gen2brain/go-fitz"
	"image/jpeg"
	"log"
	"os"
	"os/exec"
	"smart-docs/core/models"
)

type PdfWords struct {
	BBox [4]float32 `json:"bbox"`
	Text string     `json:"text"`
}

func storeImagesAndExtractPages(documentId int64) (int, [][]models.WordData, error) {
	pdfName := fmt.Sprintf("%d", documentId)
	pdfPath := fmt.Sprintf("data/%s.pdf", pdfName)
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return -1, nil, err
	}
	defer doc.Close()

	err = os.MkdirAll(fmt.Sprintf("cmd/web/assets/images/%s", pdfName), os.ModePerm)
	if err != nil {
		return -1, nil, err
	}

	pdfText, err := extractText(pdfPath)
	if err != nil {
		return -1, nil, err
	}

	for p := 0; p < doc.NumPage(); p++ {
		// This DPI must match the DPI of the python script extracting the bboxes
		img, err := doc.ImageDPI(p, 72)
		if err != nil {
			return -1, nil, err
		}

		f, err := os.Create(fmt.Sprintf("cmd/web/assets/images/%s/%d.jpg", pdfName, p))
		if err != nil {
			return -1, nil, err
		}

		err = jpeg.Encode(f, img, &jpeg.Options{Quality: jpeg.DefaultQuality})
		if err != nil {
			return -1, nil, err
		}
		_ = f.Close()
	}

	return doc.NumPage(), pdfText, nil
}

func extractText(pdfPath string) ([][]models.WordData, error) {
	cmd := exec.Command("python3", "scripts/extract_text_with_bboxes.py", pdfPath)

	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to run Python script: %v", err)
	}

	var data [][]PdfWords
	if err := json.Unmarshal(output, &data); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	pages := make([][]models.WordData, len(data))
	for i := range data {
		pages[i] = make([]models.WordData, len(data[i]))
		for j := range data[i] {
			pages[i][j] = models.WordData{
				Text: data[i][j].Text,
				Rect: models.Rect{
					X0: data[i][j].BBox[0],
					Y0: data[i][j].BBox[1],
					X1: data[i][j].BBox[2],
					Y1: data[i][j].BBox[3],
				},
			}
		}
	}

	return pages, nil
}
