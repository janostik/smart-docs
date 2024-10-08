package db

import (
	"log"
	"smart-docs/core/models"
)

func ListDocuments() ([]models.Document, error) {
	rows, err := dbInstance.db.Query(`
			select 
			    d.id, 
			    d.name,
			    d.upload_date,
			    d.status,
			    count(p.id) as page_count,
				COUNT(CASE WHEN p.status = 'VALIDATION' THEN 1 END) AS validated_count,
				COUNT(CASE WHEN p.status = 'TRAINING' THEN 1 END) AS in_progress_count
			from documents d
				left join pages p on d.id = p.document_id
			group by d.id, d.name, d.upload_date
			order by upload_date desc`)
	if err != nil {
		return nil, err
	}
	var documents []models.Document
	for rows.Next() {
		var doc models.Document
		err := rows.Scan(&doc.Id, &doc.Name, &doc.UploadDate, &doc.Status, &doc.PageCount, &doc.Validated, &doc.InProgress)
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

func LoadDocument(docId int64) (models.Document, error) {
	var doc models.Document
	err := dbInstance.db.QueryRow(`
			select 
			    d.id, 
			    d.name,
			    d.upload_date,
			    d.status,
			    count(p.id) as page_count,
				COUNT(CASE WHEN p.status = 'VALIDATION' THEN 1 END) AS validated_count,
				COUNT(CASE WHEN p.status = 'TRAINING' THEN 1 END) AS in_progress_count
			from documents d
				left join pages p on d.id = p.document_id
			where d.id = ?
			group by d.id, d.name, d.upload_date`, docId).Scan(&doc.Id, &doc.Name, &doc.UploadDate, &doc.Status, &doc.PageCount, &doc.Validated, &doc.InProgress)
	if err != nil {
		return doc, err
	}
	return doc, nil
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
