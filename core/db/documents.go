package db

import (
	"log"
	"smart-docs/core/models"
)

func ListDocuments(limit int, offset int, search string) ([]models.Document, error) {
	query := `
		select 
			d.id, 
			d.name,
			d.upload_date,
			d.status,
			d.mode,
			count(p.id) as page_count,
			COUNT(CASE WHEN p.status = 'VALIDATION' THEN 1 END) AS validated_count,
			COUNT(CASE WHEN p.status = 'TRAINING' THEN 1 END) AS in_progress_count
		from documents d
			left join pages p on d.id = p.document_id`

	args := []interface{}{}
	if search != "" {
		query += ` where d.name LIKE ?`
		args = append(args, "%"+search+"%")
	}

	query += `
		group by d.id, d.name, d.upload_date, d.status, d.mode
		order by upload_date desc
		limit ? offset ?`

	args = append(args, limit, offset)

	rows, err := dbInstance.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	var documents []models.Document
	for rows.Next() {
		var doc models.Document
		err := rows.Scan(&doc.Id, &doc.Name, &doc.UploadDate, &doc.Status, &doc.Mode, &doc.PageCount, &doc.Validated, &doc.InProgress)
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
			    d.mode,
			    d.mistral_file_id,
			    count(p.id) as page_count,
				COUNT(CASE WHEN p.status = 'VALIDATION' THEN 1 END) AS validated_count,
				COUNT(CASE WHEN p.status = 'TRAINING' THEN 1 END) AS in_progress_count
			from documents d
				left join pages p on d.id = p.document_id
			where d.id = ?
			group by d.id, d.name, d.upload_date, d.status, d.mode, d.mistral_file_id`, docId).Scan(&doc.Id, &doc.Name, &doc.UploadDate, &doc.Status, &doc.Mode, &doc.MistralFileId, &doc.PageCount, &doc.Validated, &doc.InProgress)
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
			ocr_required,
			mode
		) VALUES (?, ?, ?, ?, ?)
	`, doc.Name, doc.Status, doc.UploadDate, doc.OcrRequired, doc.Mode)
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

func UpdateMistralFileId(docId int64, fileId string) error {
	_, err := dbInstance.db.Exec(`
		update documents set mistral_file_id = ? where id=?
	`, fileId, docId)
	if err != nil {
		return err
	}
	return nil
}
