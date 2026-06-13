package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chatapp/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	MaxUploadSize = 4 << 20 // 4 MB
	UploadDir     = "./static/uploads"
)

var allowedExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".txt": true, ".zip": true, ".csv": true,
}

type UploadHandler struct{}

func NewUploadHandler() *UploadHandler {
	os.MkdirAll(UploadDir, 0755)
	return &UploadHandler{}
}

// POST /api/v1/upload — multipart form file upload, max 4MB
func (h *UploadHandler) Upload(c *gin.Context) {
	// Limit request body size to 4MB + small overhead for form fields
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize+1024)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") || strings.Contains(err.Error(), "http: request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "File too large. Maximum size is 4MB"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	if header.Size > MaxUploadSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "File too large. Maximum size is 4MB"})
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExtensions[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("File type %s is not allowed", ext)})
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

	written, err := dst.ReadFrom(file)
	if err != nil {
		os.Remove(dstPath)
		if strings.Contains(err.Error(), "too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "File too large. Maximum size is 4MB"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"

	c.JSON(http.StatusOK, gin.H{
		"url":       "/static/uploads/" + filename,
		"file_name": header.Filename,
		"file_size": written,
		"is_image":  isImage,
		"uploaded_by": userID,
	})
}
