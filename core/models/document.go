package models

import "time"

type Document struct {
	Id            int64
	Name          string
	Status        string
	UploadDate    time.Time
	OcrRequired   bool
	PageCount     int
	LocalFilePath string
}

type PendingDocument struct {
	Id     int64
	Name   string
	Status string
}
