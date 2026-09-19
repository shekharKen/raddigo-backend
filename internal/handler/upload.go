package handler

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/raddigo/raddigo/internal/utils"
)

// maxUploadImageSize caps uploaded images at 5 MiB.
const maxUploadImageSize = 5 << 20

// minUploadImageSize is the smallest a genuine image file can plausibly be;
// anything smaller is almost certainly a client sending metadata or a partial
// blob instead of the actual file content.
const minUploadImageSize = 100

// uploadImageTypes maps accepted (sniffed) content types to a file extension.
var uploadImageTypes = map[string]string{
	"image/jpeg": ".jpeg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// saveUploadedImage validates and stores the named multipart image field in the
// upload directory, returning the public URL. On failure it writes the error
// response and returns ok=false.
func saveUploadedImage(c *gin.Context, field, uploadDir, baseURL string) (string, bool) {
	fileHeader, err := c.FormFile(field)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: fmt.Sprintf("image file is required (multipart field '%s')", field)})
		return "", false
	}
	return storeImageFile(c, fileHeader, uploadDir, baseURL)
}

// saveUploadedImages validates and stores every multipart file under the named
// field (repeated form parts), returning their public URLs in submission
// order. On failure it writes the error response and returns ok=false.
func saveUploadedImages(c *gin.Context, field, uploadDir, baseURL string, maxCount int) ([]string, bool) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: fmt.Sprintf("at least one image file is required (multipart field '%s')", field)})
		return nil, false
	}
	files := form.File[field]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: fmt.Sprintf("at least one image file is required (multipart field '%s')", field)})
		return nil, false
	}
	if len(files) > maxCount {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: fmt.Sprintf("too many images: up to %d allowed", maxCount)})
		return nil, false
	}

	urls := make([]string, 0, len(files))
	for _, fileHeader := range files {
		url, ok := storeImageFile(c, fileHeader, uploadDir, baseURL)
		if !ok {
			return nil, false
		}
		urls = append(urls, url)
	}
	return urls, true
}

// storeImageFile validates a single multipart file header and stores it in
// uploadDir, returning its public URL. On failure it writes the error response
// and returns ok=false. The content type is sniffed from the file bytes and the
// stored filename is server-generated to prevent path traversal.
func storeImageFile(c *gin.Context, fileHeader *multipart.FileHeader, uploadDir, baseURL string) (string, bool) {
	if fileHeader.Size > maxUploadImageSize {
		c.JSON(http.StatusRequestEntityTooLarge, utils.ErrorResponse{Error: "image must not exceed 5 MB"})
		return "", false
	}
	if fileHeader.Size < minUploadImageSize {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: fmt.Sprintf("image file is too small to be valid (got %d bytes): the app may be sending file metadata instead of the file content", fileHeader.Size)})
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
