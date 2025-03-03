package models

import "time"

type Document struct {
	Id            int64
	Name          string
	Status        string
	UploadDate    time.Time
	OcrRequired   bool
	PageCount     int
	InProgress    int
	Validated     int
	LocalFilePath string
	IsLast        bool `json:"-"`
	Offset        int  `json:"-"`
}
