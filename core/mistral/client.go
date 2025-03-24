package mistral

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Client struct {
	apiKey string
}

type OCRResponse struct {
	Pages []Page `json:"pages"`
}

type Page struct {
	Markdown string  `json:"markdown"`
	Images   []Image `json:"images"`
}

type Image struct {
	ID          string `json:"id"`
	ImageBase64 string `json:"image_base64"`
}

func NewClient() (*Client, error) {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("MISTRAL_API_KEY environment variable is not set")
	}
	return &Client{apiKey: apiKey}, nil
}

func (c *Client) UploadFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://api.mistral.ai/v1/files", nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("error creating form file: %w", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return "", fmt.Errorf("error copying file content: %w", err)
	}

	err = writer.WriteField("purpose", "ocr")
	if err != nil {
		return "", fmt.Errorf("error writing purpose field: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return "", fmt.Errorf("error closing writer: %w", err)
	}

	req.Body = io.NopCloser(body)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error from Mistral API: status code %d", resp.StatusCode)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	return result.ID, nil
}

func (c *Client) ParseFile(fileId string) ([]string, error) {
	client := &http.Client{}

	// Get file URL
	urlReq, err := http.NewRequest("GET", fmt.Sprintf("https://api.mistral.ai/v1/files/%s/url?expiry=24", fileId), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating URL request: %w", err)
	}

	urlReq.Header.Set("Accept", "application/json")
	urlReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	urlResp, err := client.Do(urlReq)
	if err != nil {
		return nil, fmt.Errorf("error getting file URL: %w", err)
	}
	defer urlResp.Body.Close()

	if urlResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting file URL: status code %d", urlResp.StatusCode)
	}

	var urlResult struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(urlResp.Body).Decode(&urlResult); err != nil {
		return nil, fmt.Errorf("error decoding URL response: %w", err)
	}

	// Send OCR request
	ocrPayload := struct {
		Model    string `json:"model"`
		Document struct {
			Type        string `json:"type"`
			DocumentURL string `json:"document_url"`
		} `json:"document"`
		IncludeImageBase64 bool `json:"include_image_base64"`
	}{
		Model: "mistral-ocr-latest",
		Document: struct {
			Type        string `json:"type"`
			DocumentURL string `json:"document_url"`
		}{
			Type:        "document_url",
			DocumentURL: urlResult.URL,
		},
		IncludeImageBase64: true,
	}

	jsonData, err := json.Marshal(ocrPayload)
	if err != nil {
		return nil, fmt.Errorf("error marshaling OCR payload: %w", err)
	}

	ocrReq, err := http.NewRequest("POST", "https://api.mistral.ai/v1/ocr", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating OCR request: %w", err)
	}

	ocrReq.Header.Set("Content-Type", "application/json")
	ocrReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	ocrResp, err := client.Do(ocrReq)
	if err != nil {
		return nil, fmt.Errorf("error sending OCR request: %w", err)
	}
	defer ocrResp.Body.Close()

	if ocrResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error from OCR API: status code %d", ocrResp.StatusCode)
	}

	var ocrResponse OCRResponse
	if err := json.NewDecoder(ocrResp.Body).Decode(&ocrResponse); err != nil {
		return nil, fmt.Errorf("error decoding OCR response: %w", err)
	}

	// Process each page and replace image references
	markdownPages := make([]string, len(ocrResponse.Pages))
	for i, page := range ocrResponse.Pages {
		markdown := page.Markdown
		re := regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)
		matches := re.FindAllStringSubmatch(markdown, -1)

		for _, match := range matches {
			if match[1] == match[2] {
				for _, img := range page.Images {
					if strings.Contains(img.ID, match[1]) {
						imgRef := fmt.Sprintf(`<img src="%s"/>`, img.ImageBase64)
						mdRef := fmt.Sprintf("![%s](%s)", match[1], match[2])
						markdown = strings.Replace(markdown, mdRef, imgRef, 1)
					}
				}
			}
		}
		markdownPages[i] = markdown
	}

	return markdownPages, nil
}
