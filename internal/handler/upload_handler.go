package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chatapp/internal/middleware"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	MaxUploadSize = 4 << 20 // 4 MB
	UploadDir     = "./static/uploads"
)

// allowedMIME maps allowed MIME types to their canonical extension.
// Both extension AND magic-bytes MIME must match — prevents disguised uploads
// (e.g. a PHP file renamed to .jpg).
var allowedMIME = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"application/pdf": ".pdf",
	"text/plain":      ".txt",
	"text/csv":        ".csv",
	"application/zip": ".zip",
	"audio/mpeg":      ".mp3",
	"audio/ogg":       ".ogg",
	"audio/wav":       ".wav",
	"audio/webm":      ".webm",
	"video/webm":      ".webm",
	"audio/mp4":       ".m4a",
	// Office formats — mimetype detects these as zip-based; we keep extension check too
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"application/msword":   ".doc",
	"application/vnd.ms-excel": ".xls",
}

// allowedExtensions is used as a first-pass guard before reading the file.
var allowedExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".txt": true, ".zip": true, ".csv": true,
	".mp3": true, ".ogg": true, ".wav": true, ".webm": true, ".m4a": true,
}

type UploadHandler struct{}

func NewUploadHandler() *UploadHandler {
	os.MkdirAll(UploadDir, 0755)
	return &UploadHandler{}
}

// POST /api/v1/upload — multipart form file upload, max 4 MB.
// Validates both file extension AND magic-bytes MIME type to prevent
// disguised uploads (e.g. PHP script renamed to .jpg).
func (h *UploadHandler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize+1024)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "File too large. Maximum size is 4 MB"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	if header.Size > MaxUploadSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "File too large. Maximum size is 4 MB"})
		return
	}

	// First-pass: extension check (fast, no read needed)
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExtensions[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("File type %s is not allowed", ext)})
		return
	}

	// Second-pass: read magic bytes to detect actual MIME type.
	// This catches files that are disguised with a fake extension.
	// We read into a buffer so we can still write the full file afterward.
	buf, err := io.ReadAll(io.LimitReader(file, MaxUploadSize))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	detected := mimetype.Detect(buf)
	mimeStr := detected.String()
	// Strip params like "; charset=utf-8"
	if idx := strings.Index(mimeStr, ";"); idx != -1 {
		mimeStr = strings.TrimSpace(mimeStr[:idx])
	}

	if _, ok := allowedMIME[mimeStr]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("File content type '%s' is not allowed", mimeStr),
		})
		return
	}

	userID := middleware.GetUserID(c)
	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
	dstPath := filepath.Join(UploadDir, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer dst.Close()

	written, err := dst.Write(buf)
	if err != nil {
		os.Remove(dstPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	isImage := mimeStr == "image/jpeg" || mimeStr == "image/png" || mimeStr == "image/gif" || mimeStr == "image/webp"
	isAudio := mimeStr == "audio/mpeg" || mimeStr == "audio/ogg" || mimeStr == "audio/wav" ||
		mimeStr == "audio/webm" || mimeStr == "video/webm" || mimeStr == "audio/mp4"

	c.JSON(http.StatusOK, gin.H{
		"url":         "/static/uploads/" + filename,
		"file_name":   header.Filename,
		"file_size":   written,
		"is_image":    isImage,
		"is_audio":    isAudio,
		"uploaded_by": userID,
	})
}
