package handler

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/raddigo/raddigo/internal/utils"
)

// maxUploadImageSize caps uploaded images at 5 MiB.
const maxUploadImageSize = 5 << 20

// uploadImageTypes maps accepted (sniffed) content types to a file extension.
var uploadImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// saveUploadedImage validates and stores the named multipart image field in the
// upload directory, returning the public URL. On failure it writes the error
// response and returns ok=false. The content type is sniffed from the file
// bytes and the stored filename is server-generated to prevent path traversal.
func saveUploadedImage(c *gin.Context, field, uploadDir, baseURL string) (string, bool) {
	fileHeader, err := c.FormFile(field)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: fmt.Sprintf("image file is required (multipart field '%s')", field)})
		return "", false
	}
	if fileHeader.Size > maxUploadImageSize {
		c.JSON(http.StatusRequestEntityTooLarge, utils.ErrorResponse{Error: "image must not exceed 5 MB"})
		return "", false
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
		return "", false
	}
	defer file.Close()

	sniff := make([]byte, 512)
	n, _ := file.Read(sniff)
	contentType := http.DetectContentType(sniff[:n])
	ext, allowed := uploadImageTypes[contentType]
	if !allowed {
		c.JSON(http.StatusUnsupportedMediaType, utils.ErrorResponse{Error: "unsupported image type: allowed types are jpeg, png, gif and webp"})
		return "", false
	}

	filename := uuid.NewString() + ext
	dest := filepath.Join(uploadDir, filename)
	if err := c.SaveUploadedFile(fileHeader, dest); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "failed to store image"})
		return "", false
	}

	return fmt.Sprintf("%s/public/uploads/%s", baseURL, filename), true
}
