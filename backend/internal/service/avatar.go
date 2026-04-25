package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/datadir"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	AvatarTypeGenerated = "generated"
	AvatarTypeExternal  = "external"
	AvatarTypeUploaded  = "uploaded"

	DefaultAvatarStyle  = "classic_letter"
	MaxAvatarUploadSize = 1024 * 1024
	MaxAvatarURLLength  = 2048
)

var (
	AllowedAvatarTypes  = []string{AvatarTypeGenerated, AvatarTypeExternal, AvatarTypeUploaded}
	AllowedAvatarStyles = []string{DefaultAvatarStyle, "aurora_ring", "orbit_burst", "pixel_patch", "paper_cut"}

	ErrInvalidAvatarType      = infraerrors.BadRequest("INVALID_AVATAR_TYPE", "invalid avatar type")
	ErrInvalidAvatarStyle     = infraerrors.BadRequest("INVALID_AVATAR_STYLE", "invalid avatar style")
	ErrInvalidAvatarURL       = infraerrors.BadRequest("INVALID_AVATAR_URL", "avatar url must be a valid http or https url")
	ErrAvatarURLRequired      = infraerrors.BadRequest("AVATAR_URL_REQUIRED", "avatar url is required")
	ErrAvatarFileRequired     = infraerrors.BadRequest("AVATAR_FILE_REQUIRED", "avatar image file is required")
	ErrAvatarFileTooLarge     = infraerrors.BadRequest("AVATAR_FILE_TOO_LARGE", "avatar image file must be 1MB or smaller")
	ErrUnsupportedAvatarImage = infraerrors.BadRequest("UNSUPPORTED_AVATAR_IMAGE", "avatar image must be JPG, PNG, or WebP")
)

type AvatarUpload struct {
	Filename    string
	ContentType string
	Reader      io.Reader
	Size        int64
}

func normalizeAvatarType(v string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(v))
	if normalized == "" {
		return AvatarTypeGenerated, nil
	}
	if slices.Contains(AllowedAvatarTypes, normalized) {
		return normalized, nil
	}
	return "", ErrInvalidAvatarType
}

func normalizeAvatarStyle(v string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(v))
	if normalized == "" {
		return DefaultAvatarStyle, nil
	}
	if slices.Contains(AllowedAvatarStyles, normalized) {
		return normalized, nil
	}
	return "", ErrInvalidAvatarStyle
}

// AvatarTypeOrDefault returns the normalized avatar source, defaulting empty or invalid values for older rows.
func AvatarTypeOrDefault(v string) string {
	normalized, err := normalizeAvatarType(v)
	if err != nil {
		return AvatarTypeGenerated
	}
	return normalized
}

// AvatarStyleOrDefault returns the normalized generated avatar style, defaulting empty or invalid values for older rows.
func AvatarStyleOrDefault(v string) string {
	normalized, err := normalizeAvatarStyle(v)
	if err != nil {
		return DefaultAvatarStyle
	}
	return normalized
}

func normalizeExternalAvatarURL(v string) (string, error) {
	normalized := strings.TrimSpace(v)
	if normalized == "" {
		return "", ErrAvatarURLRequired
	}
	if len(normalized) > MaxAvatarURLLength {
		return "", ErrInvalidAvatarURL
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Host == "" {
		return "", ErrInvalidAvatarURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidAvatarURL
	}
	return normalized, nil
}

func storeAvatarUpload(ctx context.Context, userID int64, upload *AvatarUpload, previousURL string) (string, error) {
	if upload == nil || upload.Reader == nil {
		return "", ErrAvatarFileRequired
	}
	if upload.Size > MaxAvatarUploadSize {
		return "", ErrAvatarFileTooLarge
	}

	limited := io.LimitReader(upload.Reader, MaxAvatarUploadSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read avatar upload: %w", err)
	}
	if len(data) == 0 {
		return "", ErrAvatarFileRequired
	}
	if len(data) > MaxAvatarUploadSize {
		return "", ErrAvatarFileTooLarge
	}

	contentType := strings.ToLower(strings.TrimSpace(upload.ContentType))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}

	ext, ok := avatarExtension(contentType)
	if !ok {
		contentType = http.DetectContentType(data)
		ext, ok = avatarExtension(contentType)
		if !ok {
			return "", ErrUnsupportedAvatarImage
		}
	}

	now := time.Now().UTC()
	relDir := filepath.Join("avatars", now.Format("2006"), now.Format("01"))
	uploadsRoot := filepath.Join(datadir.Get(), "uploads")
	absDir := filepath.Join(uploadsRoot, relDir)
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return "", fmt.Errorf("create avatar directory: %w", err)
	}

	token, err := avatarRandomHex(8)
	if err != nil {
		return "", fmt.Errorf("generate avatar filename: %w", err)
	}
	filename := fmt.Sprintf("user-%d-%s.%s", userID, token, ext)
	absPath := filepath.Join(absDir, filename)
	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return "", fmt.Errorf("write avatar file: %w", err)
	}

	publicURL := "/" + filepath.ToSlash(filepath.Join("uploads", relDir, filename))
	if previousURL != "" && previousURL != publicURL {
		deleteUploadedAvatar(ctx, previousURL)
	}
	return publicURL, nil
}

func avatarExtension(contentType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/jpeg", "image/jpg":
		return "jpg", true
	case "image/png":
		return "png", true
	case "image/webp":
		return "webp", true
	default:
		return "", false
	}
}

func deleteUploadedAvatar(ctx context.Context, avatarURL string) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	relPath := uploadedAvatarRelPath(avatarURL)
	if relPath == "" {
		return
	}
	_ = os.Remove(filepath.Join(datadir.Get(), "uploads", relPath))
}

func uploadedAvatarRelPath(avatarURL string) string {
	const prefix = "/uploads/"
	if !strings.HasPrefix(avatarURL, prefix) {
		return ""
	}
	rel := strings.TrimPrefix(avatarURL, prefix)
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.HasPrefix(clean, string(filepath.Separator)) {
		return ""
	}
	if !strings.HasPrefix(filepath.ToSlash(clean), "avatars/") {
		return ""
	}
	return clean
}

func avatarRandomHex(byteLen int) (string, error) {
	if byteLen <= 0 {
		return "", errors.New("byte length must be positive")
	}
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
