package pipeline

import (
	"cloud.google.com/go/storage"
	vision "cloud.google.com/go/vision/v2/apiv1"
	"cloud.google.com/go/vision/v2/apiv1/visionpb"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"smart-docs/core/models"
	"smart-docs/core/util"
)

var (
	//googleProject   = util.Getenv("GCLOUD_PROJECT", "c-labs1")
	googleOcrBucket = util.Getenv("GCLOUD_OCR_BUCKET", "c-labs1-ocr-docs")
)

type OcrOutput struct {
	Responses []OcrResponse `json:"responses"`
}

type OcrResponse struct {
	FullTextAnnotation OcrTextAnnotation `json:"fullTextAnnotation"`
}

type OcrTextAnnotation struct {
	Pages []OcrPage `json:"pages"`
}

type OcrPage struct {
	Width  int32      `json:"width"`
	Height int32      `json:"height"`
	Blocks []OcrBlock `json:"blocks"`
}

type OcrBlock struct {
	BoundingBox OcrBbox        `json:"boundingBox"`
	Paragraphs  []OcrParagraph `json:"paragraphs"`
}

type OcrParagraph struct {
	BoundingBox OcrBbox   `json:"boundingBox"`
	Words       []OcrWord `json:"words"`
}

type OcrWord struct {
	BoundingBox OcrBbox     `json:"boundingBox"`
	Symbols     []OcrSymbol `json:"symbols"`
}

type OcrBbox struct {
	NormalizedVertices []OcrVertice `json:"normalizedVertices"`
}

type OcrVertice struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

type OcrSymbol struct {
	Text     string       `json:"text"`
	Property *OcrProperty `json:"property"`
}

type OcrProperty struct {
	DetectedBreak OcrPropertyType `json:"detectedBreak"`
}

type OcrPropertyType struct {
	Type string `json:"type"`
}

func DetectAsyncDocumentURI(docId int64, pageCount int) ([][]models.WordData, error) {

	ctx := context.Background()
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return nil, err
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer storageClient.Close()

	jobId := util.RandStringBytes(8)

	err = uploadPdf(jobId, docId, ctx, storageClient)
	if err != nil {
		return nil, err
	}

	request := &visionpb.AsyncBatchAnnotateFilesRequest{
		Requests: []*visionpb.AsyncAnnotateFileRequest{
			{
				Features: []*visionpb.Feature{
					{
						Type: visionpb.Feature_DOCUMENT_TEXT_DETECTION,
					},
				},
				InputConfig: &visionpb.InputConfig{
					GcsSource: &visionpb.GcsSource{Uri: fmt.Sprintf("gs://%s/%s/input.pdf", googleOcrBucket, jobId)},
					MimeType:  "application/pdf",
				},
				OutputConfig: &visionpb.OutputConfig{
					GcsDestination: &visionpb.GcsDestination{Uri: fmt.Sprintf("gs://%s/%s/ocr/", googleOcrBucket, jobId)},
					BatchSize:      1,
				},
			},
		},
	}

	operation, err := client.AsyncBatchAnnotateFiles(ctx, request)
	if err != nil {
		return nil, err
	}

	_, err = operation.Wait(ctx)
	if err != nil {
		return nil, err
	}

	return parseOCRResultsFromGCS(jobId, pageCount, ctx, storageClient)
}

func uploadPdf(jobId string, docId int64, ctx context.Context, client *storage.Client) error {

	bucket := client.Bucket(googleOcrBucket)
	remoteFile := bucket.Object(fmt.Sprintf("%s/input.pdf", jobId))

	file, err := os.Open(fmt.Sprintf("data/files/%d.pdf", docId))
	if err != nil {
		return err
	}

	writer := remoteFile.NewWriter(ctx)
	defer writer.Close()

	_, err = io.Copy(writer, file)
	if err != nil {
		return err
	}

	return nil
}

func parseOCRResultsFromGCS(jobId string, pageCount int, ctx context.Context, client *storage.Client) ([][]models.WordData, error) {
	log.Println(fmt.Sprintf("Parsing job: %s", jobId))

	var pages [][]models.WordData
	bkt := client.Bucket(googleOcrBucket)

	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		filename := fmt.Sprintf("%s/ocr/output-%d-to-%d.json", jobId, pageNum, pageNum)
		obj := bkt.Object(filename)
		r, err := obj.NewReader(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
		}
		defer r.Close()

		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}

		var jsonResponse OcrOutput
		if err := json.Unmarshal(data, &jsonResponse); err != nil {
			return nil, err
		}

		for _, response := range jsonResponse.Responses {

			if len(response.FullTextAnnotation.Pages) < 1 {
				log.Println(fmt.Sprintf("Skipping empty page: %d", pageNum))
				pages = append(pages, []models.WordData{})
				continue
			}

			var pageWords []models.WordData
			width := response.FullTextAnnotation.Pages[0].Width
			height := response.FullTextAnnotation.Pages[0].Height
			for _, block := range response.FullTextAnnotation.Pages[0].Blocks {
				for _, paragraph := range block.Paragraphs {
					for _, word := range paragraph.Words {

						wordText := ""
						for _, symbol := range word.Symbols {
							wordText += symbol.Text
						}

						pageWords = append(pageWords, models.WordData{
							Rect: models.Rect{
								X0: word.BoundingBox.NormalizedVertices[0].X * float32(width),
								Y0: word.BoundingBox.NormalizedVertices[0].Y * float32(height),
								X1: word.BoundingBox.NormalizedVertices[1].X * float32(width),
								Y1: word.BoundingBox.NormalizedVertices[2].Y * float32(height),
							},
							Text: wordText,
						})
					}
				}
			}
			pages = append(pages, pageWords)
		}
	}

	return pages, nil
}
