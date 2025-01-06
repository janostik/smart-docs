package pipeline

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"smart-docs/core/models"
	"smart-docs/core/util"
)

var (
	docPredictorUrl   = util.Getenv("DOC_PREDICTOR_URL", "http://localhost:10001")
	tablePredictorUrl = util.Getenv("TABLE_DETECTOR_URL", "http://localhost:10002")
)

type PredictionsResponse struct {
	Predictions []PredictionResponse `json:"predictions"`
}

type PredictionResponse struct {
	Score float32 `json:"score"`
	Label string  `json:"label"`
	X0    float32 `json:"x0"`
	X1    float32 `json:"x1"`
	Y0    float32 `json:"y0"`
	Y1    float32 `json:"y1"`
}

func GetPageDimensions(page *models.Page) error {
	imageFile, err := os.Open(fmt.Sprintf("data/images/%d/%d.jpg", page.DocumentId, page.PageNum))
	if err != nil {
		return err
	}
	defer imageFile.Close()
	image, _, err := image.DecodeConfig(imageFile)
	if err != nil {
		return err
	}
	page.Width = image.Width
	page.Height = image.Height
	return nil
}

func RunDetectionOnPage(docId int64, page int) ([]models.Prediction, error) {

	imageFile, err := os.Open(fmt.Sprintf("data/images/%d/%d.jpg", docId, page))
	if err != nil {
		return nil, err
	}
	defer imageFile.Close()

	imageData, err := io.ReadAll(imageFile)
	if err != nil {
		log.Println(fmt.Sprintf("failed file read: \n%+v", err))
		return nil, err
	}

	imageFile.Seek(0, 0)
	img, _, err := image.Decode(imageFile)
	if err != nil {
		log.Println(fmt.Sprintf("Failed decode: \n%+v", err))
		return nil, err
	}

	predictions := make([]models.Prediction, 0)
	docPredictions := runPrediction(imageData, docPredictorUrl)
	for _, p := range docPredictions {
		prediction := models.Prediction{
			Score: p.Score,
			Label: p.Label,
			Rect: models.Rect{
				X0: p.X0,
				X1: p.X1,
				Y0: p.Y0,
				Y1: p.Y1,
			},
		}
		if p.Label == "table" {
			cropped, err := cropImage(img, image.Rect(int(p.X0), int(p.Y0), int(p.X1), int(p.Y1)))
			if err != nil {
				log.Println(fmt.Sprintf("Error detecting segments: \n%+v", err))
			} else {
				tablePredictions := runPrediction(cropped, tablePredictorUrl)
				if len(tablePredictions) == 0 {
					prediction.Label = "paragraph"
				}
				for _, t := range tablePredictions {
					prediction.Table = append(prediction.Table, models.Prediction{
						Score: t.Score,
						Label: t.Label,
						Table: make([]models.Prediction, 0),
						Rect: models.Rect{
							X0: t.X0,
							X1: t.X1,
							Y0: t.Y0,
							Y1: t.Y1,
						},
					})
				}
			}
		}
		predictions = append(predictions, prediction)
	}

	yCmp := func(a, b models.Prediction) int {
		return cmp.Compare(a.Y0, b.Y0)
	}
	slices.SortFunc(predictions, yCmp)

	return predictions, nil
}

func runPrediction(image []byte, predictorUrl string) []PredictionResponse {
	imageBase64 := base64.StdEncoding.EncodeToString(image)
	type PredictionRequest struct {
		ImageB64 string `json:"image_b64"`
	}
	requestJson, err := json.Marshal(PredictionRequest{ImageB64: imageBase64})
	if err != nil {
		log.Println(fmt.Sprintf("Error detecting segments: \n%+v", err))
		return make([]PredictionResponse, 0)
	}
	res, err := http.Post(fmt.Sprintf("%s/predict", predictorUrl), "application/json", bytes.NewBuffer(requestJson))
	if err != nil {
		log.Println(fmt.Sprintf("Error detecting segments: \n%+v", err))
		return make([]PredictionResponse, 0)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	var responseData PredictionsResponse
	err = json.Unmarshal(body, &responseData)
	if err != nil {
		log.Println(fmt.Sprintf("Error detecting segments: \n%+v", err))
		return make([]PredictionResponse, 0)
	}
	return responseData.Predictions
}

func cropImage(img image.Image, crop image.Rectangle) ([]byte, error) {
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}

	simg, ok := img.(subImager)
	if !ok {
		return nil, fmt.Errorf("image does not support cropping")
	}

	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, simg.SubImage(crop), nil)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
