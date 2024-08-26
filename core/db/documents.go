package db

import (
	"log"
	"smart-docs/core/models"
)

func ListDocuments() ([]models.Document, error) {
	rows, err := dbInstance.db.Query(`select id, name, status, upload_date, ocr_required from documents order  by upload_date desc`)
	if err != nil {
		return nil, err
	}
	var documents []models.Document
	for rows.Next() {
		var doc models.Document
		err := rows.Scan(&doc.Id, &doc.Name, &doc.Status, &doc.UploadDate, &doc.OcrRequired)
		if err != nil {
			return nil, err
		}
		documents = append(documents, doc)
	}

	err = rows.Close()
	if err != nil {
		log.Fatal(err)
	}

	return documents, nil
}

func StoreDocument(doc *models.Document) (int64, error) {

	res, err := dbInstance.db.Exec(`
		INSERT INTO documents (
			name,
			status,
			upload_date,
			ocr_required
		) VALUES (?, ?, ?, ?)
	`, doc.Name, doc.Status, doc.UploadDate, doc.OcrRequired)
	if err != nil {
		return -1, err
	}
	doc.Id, _ = res.LastInsertId()
	return doc.Id, nil
}

func UpdateDocumentStatus(docId int64, status string) error {
	_, err := dbInstance.db.Exec(`
		update documents set status = ? where id=?
	`, status, docId)
	if err != nil {
		return err
	}
	return nil
}
