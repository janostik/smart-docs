package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"smart-docs/cmd/web"
	"smart-docs/core/auth"
	"smart-docs/core/db"
	"smart-docs/core/models"
	"smart-docs/core/pipeline"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var tmpl *template.Template

func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Handle("/assets/*", http.FileServer(http.FS(web.Files)))

	imgFs := http.FileServer(http.Dir("./data/images/"))
	r.Handle("/images/*", http.StripPrefix("/images/", imgFs))

	r.Get("/health", s.healthHandler)
	r.Get("/", s.ListDocuments)
	r.Get("/annotate", s.NextPageToAnnotate)
	r.Get("/annotate/{documentId}/{pageNum}", s.AnnotatePage)
	r.Post("/upload", s.UploadDocument)
	r.Get("/document/{documentId}", s.LoadDocument)
	r.Delete("/document/{documentId}", s.DeleteDocument)
	r.Get("/document/{documentId}/content", s.LoadContent)
	r.Put("/document/{documentId}/retry", s.Retry)
	r.Patch("/document/{documentId}/{pageNum}/status/{newStatus}", s.UpdateStatus)
	r.Get("/document/{documentId}/{pageNum}/predictions", s.GetPredictions)
	r.Post("/document/{documentId}/{pageNum}/predictions", s.SetPredictions)

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}

	tmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(web.Files,
		"templates/documents.go.html",
		"templates/annotate.go.html",
		"templates/document.go.html",
		"templates/document-loading.go.html",
		"templates/nothing-to-annotate.go.html",
		"templates/login.go.html",
		"templates/partial/head.go.html",
		"templates/partial/page-status.go.html",
		"templates/partial/document-rows.go.html",
	))

	return auth.AuthMiddleware(tmpl, r)
}

func (s *Server) LoadDocument(w http.ResponseWriter, r *http.Request) {
	docId, err := strconv.ParseInt(chi.URLParam(r, "documentId"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	pageNum := 0
	if r.URL.Query().Has("page") {
		pageNum, err = strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	doc, err := db.LoadDocument(docId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("hx-current-url") != "" && !strings.Contains(r.Header.Get("hx-current-url"), "document") {
		data := struct {
			Documents []models.Document
			Offset    int
			Search    string
		}{
			Documents: []models.Document{doc},
			Offset:    0,
			Search:    "",
		}
		err = tmpl.ExecuteTemplate(w, "document-rows.go.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if doc.Status == "PROCESSING" {
		err = tmpl.ExecuteTemplate(w, "document-loading.go.html", doc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	var pageView models.PageView
	err = db.LoadPage(docId, pageNum, &pageView)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "document.go.html", pageView)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	docId, err := strconv.ParseInt(chi.URLParam(r, "documentId"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	err = db.DeleteDocument(docId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	docId, err := strconv.ParseInt(chi.URLParam(r, "documentId"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	pageNum, err := strconv.Atoi(chi.URLParam(r, "pageNum"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	newStatus := chi.URLParam(r, "newStatus")
	if newStatus != "TRAINING" && newStatus != "PREDICTION" && newStatus != "VALIDATION" {
		http.Error(w, "Invalid status", http.StatusBadRequest)
	}

	var pageView models.PageView
	err = db.UpdateTrainingStatus(docId, pageNum, newStatus)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	pageView.Status = newStatus
	err = tmpl.ExecuteTemplate(w, "page-status.go.html", pageView)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) LoadContent(w http.ResponseWriter, r *http.Request) {
	docId, err := strconv.ParseInt(chi.URLParam(r, "documentId"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	content, err := db.GetPdfDocText(docId)
	if err != nil {
		http.Error(w, "Failed to export document content", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8\n")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(content))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) UploadDocument(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		log.Println(fmt.Sprintf("Failed to parse request form: \n%v", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shouldRunOcr := r.FormValue("ocr") == "on"
	mode := r.FormValue("mode")
	file, handler, err := r.FormFile("file")
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	doc := models.Document{
		Id:          -1,
		Name:        handler.Filename,
		UploadDate:  time.Now(),
		OcrRequired: false,
		Status:      "PROCESSING",
		Mode:        mode,
	}
	docId, err := db.StoreDocument(&doc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filePath := fmt.Sprintf("./data/files/%d.pdf", docId)
	dest, err := os.Create(filePath)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dest.Close()

	_, err = io.Copy(dest, file)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go pipeline.ProcessPdf(docId, shouldRunOcr, mode)

	w.Header().Add("HX-Redirect", fmt.Sprintf("/document/%d", doc.Id))
}

func (s *Server) ListDocuments(w http.ResponseWriter, r *http.Request) {
	limit := 10
	offset := 0
	search := ""

	if r.URL.Query().Has("offset") {
		parsedOffset, err := strconv.Atoi(r.URL.Query().Get("offset"))
		if err == nil {
			offset = parsedOffset
		}
	}

	if r.URL.Query().Has("search") {
		search = r.URL.Query().Get("search")
	}

	documents, err := db.ListDocuments(limit, offset, search)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If no documents returned or we have fewer documents than limit (meaning we've reached the end)
	if (len(documents) == 0 || len(documents) < limit) && r.URL.Query().Has("offset") {
		w.Header().Set("HX-Trigger", "endOfResults")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	data := struct {
		Documents []models.Document
		Offset    int
		Search    string
	}{
		Documents: documents,
		Offset:    offset,
		Search:    search,
	}

	template := "documents.go.html"
	if r.Header.Get("HX-Trigger") == "search" || r.URL.Query().Has("offset") || r.URL.Query().Has("search") {
		template = "document-rows.go.html"
	}

	err = tmpl.ExecuteTemplate(w, template, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) NextPageToAnnotate(w http.ResponseWriter, r *http.Request) {
	var page models.PageView
	err := db.NextPageToAnnotate(&page)

	if err == sql.ErrNoRows {
		err = tmpl.ExecuteTemplate(w, "nothing-to-annotate.go.html", page)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	r.Header.Add("Cache-Control", "no-store")
	http.Redirect(w, r, fmt.Sprintf("/annotate/%d/%d", page.DocumentId, page.PageNum), 307)
}

func (s *Server) AnnotatePage(w http.ResponseWriter, r *http.Request) {
	docId, err := strconv.ParseInt(chi.URLParam(r, "documentId"), 10, 64)
	pageNum, err := strconv.Atoi(chi.URLParam(r, "pageNum"))

	var page models.PageView
	err = db.LoadPage(docId, pageNum, &page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.ExecuteTemplate(w, "annotate.go.html", page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) GetPredictions(w http.ResponseWriter, r *http.Request) {
	docId, err := strconv.ParseInt(chi.URLParam(r, "documentId"), 10, 64)
	pageNum, err := strconv.Atoi(chi.URLParam(r, "pageNum"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	predictions, err := db.GetPredictions(docId, pageNum)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(predictions))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) SetPredictions(w http.ResponseWriter, r *http.Request) {
	docId, err := strconv.ParseInt(chi.URLParam(r, "documentId"), 10, 64)
	pageNum, err := strconv.Atoi(chi.URLParam(r, "pageNum"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var predictions []models.Prediction
	err = json.NewDecoder(r.Body).Decode(&predictions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pdfText, err := db.GetPdfPageText(docId, pageNum)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var htmlText = pipeline.ParseHtmlAndAdjustDetection(&pdfText, &predictions, docId, pageNum)
	err = db.UpdatePredictionsAndText(docId, pageNum, &predictions, &htmlText)
	pipeline.DrawBoundingBoxes(docId, pageNum, &predictions, "prediction")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	jsonResp, err := json.Marshal(predictions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = w.Write(jsonResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) Retry(w http.ResponseWriter, r *http.Request) {
	docId, err := strconv.ParseInt(chi.URLParam(r, "documentId"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pipeline.RetryAnnotations(docId)

	doc, err := db.LoadDocument(docId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Documents []models.Document
		Offset    int
		Search    string
	}{
		Documents: []models.Document{doc},
		Offset:    0,
		Search:    "",
	}
	err = tmpl.ExecuteTemplate(w, "document-rows.go.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	jsonResp, _ := json.Marshal(s.db.Health())
	_, _ = w.Write(jsonResp)
}
