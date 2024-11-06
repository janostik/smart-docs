package pipeline

import (
	"cloud.google.com/go/storage"
	vision "cloud.google.com/go/vision/v2/apiv1"
	"cloud.google.com/go/vision/v2/apiv1/visionpb"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func DetectAsyncDocumentURI(docId int64) ([][]models.WordData, error) {

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

	err = ensurePdfUploaded(docId, ctx, storageClient)
	if err != nil {
		return nil, err
	}

	// Hardcoded delay, otherwise google vision doesn't know about the newly created file yet.
	//time.Sleep(15 * time.Second)

	request := &visionpb.AsyncBatchAnnotateFilesRequest{
		Requests: []*visionpb.AsyncAnnotateFileRequest{
			{
				Features: []*visionpb.Feature{
					{
						Type: visionpb.Feature_DOCUMENT_TEXT_DETECTION,
					},
				},
				InputConfig: &visionpb.InputConfig{
					GcsSource: &visionpb.GcsSource{Uri: fmt.Sprintf("gs://%s/%d/input.pdf", googleOcrBucket, docId)},
					MimeType:  "application/pdf",
				},
				OutputConfig: &visionpb.OutputConfig{
					GcsDestination: &visionpb.GcsDestination{Uri: fmt.Sprintf("gs://%s/%d/ocr/", googleOcrBucket, docId)},
					BatchSize:      2,
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

	return parseOCRResultsFromGCS(docId, ctx, storageClient)
}

func ensurePdfUploaded(docId int64, ctx context.Context, client *storage.Client) error {

	bucket := client.Bucket(googleOcrBucket)
	remoteFile := bucket.Object(fmt.Sprintf("%d/input.pdf", docId))
	_, err := remoteFile.Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		file, err := os.Open(fmt.Sprintf("data/files/%d.pdf", docId))
		if err != nil {
			return err
		}

		writer := remoteFile.NewWriter(ctx)

		_, err = io.Copy(writer, file)
		if err != nil {
			return err
		}

		writer.Close()
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

func parseOCRResultsFromGCS(docId int64, ctx context.Context, client *storage.Client) ([][]models.WordData, error) {
	var pages [][]models.WordData

	bkt := client.Bucket(googleOcrBucket)

	it := bkt.Objects(ctx, &storage.Query{Prefix: fmt.Sprintf("%d/ocr/", docId)})
	for {
		objAttrs, err := it.Next()
		if err != nil {
			break
		}

		obj := bkt.Object(objAttrs.Name)
		r, err := obj.NewReader(ctx)
		if err != nil {
			return nil, err
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

		for _, page := range jsonResponse.Responses {
			var pageWords []models.WordData
			for _, block := range page.FullTextAnnotation.Pages[0].Blocks {
				for _, paragraph := range block.Paragraphs {
					for _, word := range paragraph.Words {

						wordText := ""
						for _, symbol := range word.Symbols {
							wordText += symbol.Text
						}

						pageWords = append(pageWords, models.WordData{
							Rect: models.Rect{
								X0: word.BoundingBox.NormalizedVertices[0].X,
								Y0: word.BoundingBox.NormalizedVertices[0].Y,
								X1: word.BoundingBox.NormalizedVertices[2].X,
								Y1: word.BoundingBox.NormalizedVertices[2].Y,
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
