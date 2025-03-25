package pipeline

import (
	"encoding/json"
	"fmt"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"smart-docs/core/models"
	"smart-docs/scripts"

	"github.com/gen2brain/go-fitz"
)

type PdfWords struct {
	BBox [4]float32 `json:"bbox"`
	Text string     `json:"text"`
}

func storeImagesAndExtractPages(documentId int64) (int, [][]models.WordData, error) {
	pdfName := fmt.Sprintf("%d", documentId)
	pdfPath := fmt.Sprintf("./data/files/%s.pdf", pdfName)
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return -1, nil, err
	}
	defer doc.Close()

	err = os.MkdirAll(fmt.Sprintf("./data/images/%s", pdfName), os.ModePerm)
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

		f, err := os.Create(fmt.Sprintf("./data/images/%s/%d.jpg", pdfName, p))
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
	scriptContent, err := scripts.Files.ReadFile("extract_text_with_bboxes.py")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded script: %v", err)
	}

	tmpDir := os.TempDir()
	tmpScript := filepath.Join(tmpDir, "extract_text_with_bboxes.py")

	if err := os.WriteFile(tmpScript, scriptContent, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temporary script: %v", err)
	}
	defer os.Remove(tmpScript)

	cmd := exec.Command("python3", tmpScript, pdfPath)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run Python script: %v", err)
	}

	var data [][]PdfWords
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
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
