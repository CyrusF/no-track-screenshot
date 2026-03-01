package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/nfnt/resize"
)

type Handler struct {
	db        *DB
	ai        *AIClient
	cfg       *Config
	prompt    string
	templates *template.Template
}

func NewHandler(db *DB, ai *AIClient, cfg *Config) (*Handler, error) {
	promptBytes, err := os.ReadFile("prompt.txt")
	if err != nil {
		return nil, fmt.Errorf("read prompt.txt: %w", err)
	}

	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Handler{
		db:        db,
		ai:        ai,
		cfg:       cfg,
		prompt:    strings.TrimSpace(string(promptBytes)),
		templates: tmpl,
	}, nil
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.handleIndex)
	mux.HandleFunc("POST /login", h.handleLogin)
	mux.HandleFunc("POST /upload", h.handleUpload)
	mux.HandleFunc("GET /task/{id}", h.handleTask)
	mux.HandleFunc("POST /task/{id}/auth", h.handleTaskAuth)
	mux.HandleFunc("GET /task/{id}/status", h.handleTaskStatus)
	mux.HandleFunc("GET /task/{id}/preview", h.handleTaskPreview)
}

// --- Auth helpers ---

func hashPassword(pw string) string {
	h := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(h[:])
}

func (h *Handler) isGlobalAuthed(r *http.Request) bool {
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		return false
	}
	return cookie.Value == hashPassword(h.cfg.Password)
}

func (h *Handler) isTaskAuthed(r *http.Request, taskID string) bool {
	cookie, err := r.Cookie("task_" + taskID)
	if err != nil {
		return false
	}
	return cookie.Value == "granted"
}

func (h *Handler) setGlobalAuth(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    hashPassword(h.cfg.Password),
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400 * 7,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) setTaskAuth(w http.ResponseWriter, taskID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "task_" + taskID,
		Value:    "granted",
		Path:     "/task/" + taskID,
		HttpOnly: true,
		MaxAge:   86400,
		SameSite: http.SameSiteLaxMode,
	})
}

// --- Handlers ---

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if !h.isGlobalAuthed(r) {
		h.templates.ExecuteTemplate(w, "login.html", nil)
		return
	}
	h.templates.ExecuteTemplate(w, "index.html", nil)
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	password := r.FormValue("password")
	if password != h.cfg.Password {
		h.templates.ExecuteTemplate(w, "login.html", map[string]string{"Error": "密码错误"})
		return
	}
	h.setGlobalAuth(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if !h.isGlobalAuthed(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.ParseMultipartForm(32 << 20) // 32MB max
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "No image uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	imgData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read image", http.StatusInternalServerError)
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/png"
	}

	preview, err := generatePreview(imgData)
	if err != nil {
		log.Printf("preview generation failed: %v", err)
		preview = nil
	}

	taskID, _ := gonanoid.New(10)
	taskPassword := r.FormValue("task_password")

	now := time.Now()
	task := &Task{
		ID:           taskID,
		Status:       StatusPending,
		Preview:      preview,
		TaskPassword: taskPassword,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.db.CreateTask(task); err != nil {
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	imgBase64 := base64.StdEncoding.EncodeToString(imgData)

	go h.processTask(taskID, imgBase64, mimeType)

	http.Redirect(w, r, "/task/"+taskID, http.StatusSeeOther)
}

func (h *Handler) processTask(taskID, imgBase64, mimeType string) {
	h.db.UpdateTaskResult(taskID, StatusProcessing, "", "")

	html, err := h.ai.GenerateHTML(imgBase64, mimeType, h.prompt)
	if err != nil {
		log.Printf("task %s AI error: %v", taskID, err)
		h.db.UpdateTaskResult(taskID, StatusFailed, "", err.Error())
		return
	}

	h.db.UpdateTaskResult(taskID, StatusDone, html, "")
	log.Printf("task %s completed", taskID)
}

func (h *Handler) handleTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := h.db.GetTask(id)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if !h.isGlobalAuthed(r) && !h.isTaskAuthed(r, id) {
		h.templates.ExecuteTemplate(w, "password.html", map[string]string{"TaskID": id})
		return
	}

	data := map[string]interface{}{
		"Task":    task,
		"TaskHTML": template.HTML(task.HTML),
	}
	h.templates.ExecuteTemplate(w, "task.html", data)
}

func (h *Handler) handleTaskAuth(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	password := r.FormValue("password")

	if password == h.cfg.Password {
		h.setGlobalAuth(w)
		h.setTaskAuth(w, id)
		http.Redirect(w, r, "/task/"+id, http.StatusSeeOther)
		return
	}

	task, err := h.db.GetTask(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if task.TaskPassword != "" && password == task.TaskPassword {
		h.setTaskAuth(w, id)
		http.Redirect(w, r, "/task/"+id, http.StatusSeeOther)
		return
	}

	h.templates.ExecuteTemplate(w, "password.html", map[string]string{
		"TaskID": id,
		"Error":  "密码错误",
	})
}

func (h *Handler) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := h.db.GetTask(id)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     task.ID,
		"status": task.Status,
		"error":  task.Error,
	})
}

func (h *Handler) handleTaskPreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := h.db.GetTask(id)
	if err != nil || len(task.Preview) == 0 {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(task.Preview)
}

func generatePreview(imgData []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	resized := resize.Resize(32, 32, img, resize.Lanczos3)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 60}); err != nil {
		return nil, fmt.Errorf("encode preview: %w", err)
	}

	return buf.Bytes(), nil
}
