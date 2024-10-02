package db

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"smart-docs/core/models"
	"strings"
)

func UpdatePageCount(docId int64, pageCount int) error {
	_, err := dbInstance.db.Exec(`
		update documents set page_count = ? where id=?
	`, pageCount, docId)
	if err != nil {
		return err
	}
	return nil
}

func StorePages(pages *[]models.Page) error {
	stmt, err := dbInstance.db.Prepare(`
		INSERT INTO pages (
			document_id,
			page_num,
			pdf_text,
		    ocr_text,
		    status,
			predictions,
			html
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	for _, page := range *pages {

		serialisedPredictions, err := json.Marshal(page.Predictions)
		if err != nil {
			log.Panicln(fmt.Sprintf("Cannot serialise predictions: \n%+v", err))
		}

		serialisedWords, err := json.Marshal(page.PdfText)
		if err != nil {
			log.Println(fmt.Sprintf("Error serialising pdf bboxes: \n%+v", err))
		}

		_, err = stmt.Exec(page.DocumentId, page.PageNum, serialisedWords, page.OcrText, page.Status, serialisedPredictions, page.Html)
		if err != nil {
			return err
		}
	}

	return nil
}

func LoadDocument(docId int64) (models.PendingDocument, error) {
	var document models.PendingDocument
	err := dbInstance.db.QueryRow(`
		select 
		    doc.id, 
		    doc.name, 
		    doc.status 
		from documents doc where doc.id = ?
	`, docId).Scan(
		&document.Id,
		&document.Name,
		&document.Status,
	)
	if err != nil {
		return models.PendingDocument{}, err
	}
	return document, nil
}

func DeleteDocument(docId int64) error {
	_, err := dbInstance.db.Exec(`delete from pages where document_id = ?`, docId)
	if err != nil {
		return err
	}
	_, err = dbInstance.db.Exec(`delete from documents where id = ?`, docId)
	if err != nil {
		return err
	}
	return nil
}

func NextPageToAnnotate(page *models.PageView) error {
	err := dbInstance.db.QueryRow(`
		select 
		    p.id, 
		    doc.name,
		    doc.id,
		    p.status,
		    p.page_num,
		    p.page_num - 1,
		    p.page_num + 1,
		    coalesce(p.page_num > 0, false),
		    coalesce(doc.page_count - 1 > p.page_num, false)
		from documents doc
			left join pages p on doc.id = p.document_id
		where p.status = 'TRAINING'
	`).Scan(
		&page.Id,
		&page.DocumentName,
		&page.DocumentId,
		&page.Status,
		&page.PageNum,
		&page.PreviousPage,
		&page.NextPage,
		&page.HasPreviousPage,
		&page.HasNextPage,
	)
	if err != nil {
		return err
	}
	return nil
}

func LoadPage(docId int64, pageNum int, page *models.PageView) error {
	var htmlString string
	err := dbInstance.db.QueryRow(`
		select 
		    p.id, 
		    doc.name,
		    doc.id,
		    p.status,
		    p.html,
		    p.page_num,
		    p.page_num - 1,
		    p.page_num + 1,
		    coalesce(p.page_num > 0, false),
		    coalesce(doc.page_count - 1 > p.page_num, false)
		from documents doc
			left join pages p on doc.id = p.document_id and p.page_num = ?
		where doc.id = ?
	`, pageNum, docId).Scan(
		&page.Id,
		&page.DocumentName,
		&page.DocumentId,
		&page.Status,
		&htmlString,
		&page.PageNum,
		&page.PreviousPage,
		&page.NextPage,
		&page.HasPreviousPage,
		&page.HasNextPage,
	)
	if err != nil {
		return err
	}
	page.Html = template.HTML(htmlString)
	return nil
}

func GetPredictions(docId int64, pageNum int) (string, error) {
	var serialisedPredictions string
	err := dbInstance.db.QueryRow(`
		select 
		    p.predictions
		from documents doc
			left join pages p on doc.id = p.document_id and p.page_num = ?
		where doc.id = ?
	`, pageNum, docId).Scan(
		&serialisedPredictions,
	)
	if err != nil {
		return "", err
	}
	return serialisedPredictions, nil
}

func GetPdfPageText(docId int64, pageNum int) ([]models.WordData, error) {
	var serialisedText string
	err := dbInstance.db.QueryRow(`
		select 
		    p.pdf_text
		from documents doc
			left join pages p on doc.id = p.document_id and p.page_num = ?
		where doc.id = ?
	`, pageNum, docId).Scan(
		&serialisedText,
	)
	if err != nil {
		return nil, err
	}

	var words []models.WordData
	err = json.Unmarshal([]byte(serialisedText), &words)
	if err != nil {
		return nil, err
	}

	return words, nil
}

func GetPdfDocText(docId int64) (string, error) {
	var pages []string
	rows, err := dbInstance.db.Query(`
    SELECT 
        p.html
    FROM documents doc
    LEFT JOIN pages p ON doc.id = p.document_id
    WHERE doc.id = ?
`, docId)
	if err != nil {
		log.Println(fmt.Sprintf("query failed: %v", err))
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var html string
		if err := rows.Scan(&html); err != nil {
			log.Println(fmt.Sprintf("scan failed: %v", err))
			return "", err
		}
		pages = append(pages, html)
	}

	if err := rows.Err(); err != nil {
		log.Println(fmt.Sprintf("row iteration failed: %v", err))
		return "", err
	}
	var b strings.Builder
	for p, _ := range pages {
		b.WriteString(fmt.Sprintf("\n<section page=\"%d\">\n%s\n</section>\n", p, pages[p]))
	}

	return b.String(), nil
}

func UpdateTrainingStatus(docId int64, pageNum int, status string) error {
	_, err := dbInstance.db.Exec(`
		update pages set status = ? where document_id=? and page_num=?
	`, status, docId, pageNum)
	if err != nil {
		return err
	}
	return nil
}

func UpdatePredictionsAndText(docId int64, pageNum int, predictions *[]models.Prediction, html *string) error {
	serialisedPredictions, err := json.Marshal(*predictions)
	if err != nil {
		return err
	}
	_, err = dbInstance.db.Exec(`
		update pages 
		set predictions = ?, html = ?
		where document_id=? and page_num=?
	`, string(serialisedPredictions), html, docId, pageNum)
	if err != nil {
		return err
	}
	return nil
}
