package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// maxProfileImageSize caps uploaded profile images at 5 MiB.
const maxProfileImageSize = 5 << 20

// allowedImageTypes maps accepted (sniffed) content types to a file extension.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// profileUserService abstracts user profile retrieval and updates.
type profileUserService interface {
	GetProfile(ctx context.Context, id string) (model.User, error)
	UpdateProfile(ctx context.Context, id string, in dto.UpdateUserProfileRequest) (model.User, error)
	SetProfileImage(ctx context.Context, id, imageURL string) (model.User, error)
}

// profilePartnerService abstracts partner profile retrieval and updates.
type profilePartnerService interface {
	GetProfile(ctx context.Context, id string) (model.Partner, error)
	UpdateProfile(ctx context.Context, id string, in dto.UpdatePartnerProfileRequest) (model.Partner, error)
	SetProfileImage(ctx context.Context, id, imageURL string) (model.Partner, error)
}

// ProfileHandler exposes profile endpoints for users and partners, including
// profile-image uploads served from the public directory.
type ProfileHandler struct {
	users     profileUserService
	partners  profilePartnerService
	uploadDir string
	baseURL   string
}

// NewProfileHandler creates a ProfileHandler. uploadDir is the directory where
// images are written; publicBaseURL is the externally reachable base URL used
// to build the stored image URL.
func NewProfileHandler(users profileUserService, partners profilePartnerService, uploadDir, publicBaseURL string) *ProfileHandler {
	return &ProfileHandler{
		users:     users,
		partners:  partners,
		uploadDir: uploadDir,
		baseURL:   publicBaseURL,
	}
}

// GetUserProfile handles GET /api/v1/user/:userId/profile.
func (h *ProfileHandler) GetUserProfile(c *gin.Context) {
	user, err := h.users.GetProfile(c.Request.Context(), c.Param("userId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// UpdateUserProfile handles PUT /api/v1/user/:userId/profile.
func (h *ProfileHandler) UpdateUserProfile(c *gin.Context) {
	var in dto.UpdateUserProfileRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	user, err := h.users.UpdateProfile(c.Request.Context(), c.Param("userId"), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// UploadUserImage handles POST /api/v1/user/:userId/profile/image.
func (h *ProfileHandler) UploadUserImage(c *gin.Context) {
	imageURL, ok := h.storeUploadedImage(c)
	if !ok {
		return
	}
	user, err := h.users.SetProfileImage(c.Request.Context(), c.Param("userId"), imageURL)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// GetPartnerProfile handles GET /api/v1/partner/:partnerId/profile.
func (h *ProfileHandler) GetPartnerProfile(c *gin.Context) {
	partner, err := h.partners.GetProfile(c.Request.Context(), c.Param("partnerId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"partner": partner})
}

// UpdatePartnerProfile handles PUT /api/v1/partner/:partnerId/profile.
func (h *ProfileHandler) UpdatePartnerProfile(c *gin.Context) {
	var in dto.UpdatePartnerProfileRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	partner, err := h.partners.UpdateProfile(c.Request.Context(), c.Param("partnerId"), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"partner": partner})
}

// UploadPartnerImage handles POST /api/v1/partner/:partnerId/profile/image.
func (h *ProfileHandler) UploadPartnerImage(c *gin.Context) {
	imageURL, ok := h.storeUploadedImage(c)
	if !ok {
		return
	}
	partner, err := h.partners.SetProfileImage(c.Request.Context(), c.Param("partnerId"), imageURL)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"partner": partner})
}

// storeUploadedImage validates and saves the "image" multipart file to the
// upload directory, returning the public URL. On failure it writes the error
// response and returns ok=false.
func (h *ProfileHandler) storeUploadedImage(c *gin.Context) (string, bool) {
	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "image file is required (multipart field 'image')"})
		return "", false
	}
	if fileHeader.Size > maxProfileImageSize {
		c.JSON(http.StatusRequestEntityTooLarge, utils.ErrorResponse{Error: "image must not exceed 5 MB"})
		return "", false
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
		return "", false
	}
	defer file.Close()

	// Sniff the content type from the bytes rather than trusting the filename or
	// the client-supplied Content-Type header.
	sniff := make([]byte, 512)
	n, _ := file.Read(sniff)
	contentType := http.DetectContentType(sniff[:n])
	ext, allowed := allowedImageTypes[contentType]
	if !allowed {
		c.JSON(http.StatusUnsupportedMediaType, utils.ErrorResponse{Error: "unsupported image type: allowed types are jpeg, png, gif and webp"})
		return "", false
	}
	if _, err := file.Seek(0, 0); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
		return "", false
	}

	// A server-generated name plus a fixed directory prevents path traversal and
	// filename collisions.
	filename := uuid.NewString() + ext
	dest := filepath.Join(h.uploadDir, filename)
	if err := c.SaveUploadedFile(fileHeader, dest); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "failed to store image"})
		return "", false
	}

	return fmt.Sprintf("%s/public/uploads/%s", h.baseURL, filename), true
}

// writeServiceError maps domain errors to HTTP responses.
func (h *ProfileHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, utils.ErrValidation):
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: err.Error()})
	case errors.Is(err, utils.ErrNotFound):
		c.JSON(http.StatusNotFound, utils.ErrorResponse{Error: "resource not found"})
	default:
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
	}
}
