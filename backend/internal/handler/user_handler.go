package handler

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UserHandler handles user-related requests
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// ChangePasswordRequest represents the change password request payload
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// UpdateProfileRequest represents the update profile request payload
type UpdateProfileRequest struct {
	Username    *string `json:"username" form:"username"`
	AvatarType  *string `json:"avatar_type" form:"avatar_type"`
	AvatarStyle *string `json:"avatar_style" form:"avatar_style"`
	AvatarURL   *string `json:"avatar_url" form:"avatar_url"`
}

// GetProfile handles getting user profile
// GET /api/v1/users/me
func (h *UserHandler) GetProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	userData, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromService(userData))
}

// ChangePassword handles changing user password
// POST /api/v1/users/me/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.ChangePasswordRequest{
		CurrentPassword: req.OldPassword,
		NewPassword:     req.NewPassword,
	}
	err := h.userService.ChangePassword(c.Request.Context(), subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Password changed successfully"})
}

// UpdateProfile handles updating user profile
// PUT /api/v1/users/me
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req UpdateProfileRequest
	var avatarFile *service.AvatarUpload
	var err error
	if isMultipartRequest(c.GetHeader("Content-Type")) {
		if err = c.Request.ParseMultipartForm(service.MaxAvatarUploadSize + 1024*1024); err == nil {
			if c.Request.MultipartForm != nil {
				defer func() { _ = c.Request.MultipartForm.RemoveAll() }()
			}
			avatarFile, err = avatarUploadFromForm(c.Request.MultipartForm)
		}
	} else {
		err = c.ShouldBindJSON(&req)
	}
	if err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if isMultipartRequest(c.GetHeader("Content-Type")) {
		bindMultipartProfileRequest(c, &req)
	}

	svcReq := service.UpdateProfileRequest{
		Username:    req.Username,
		AvatarType:  req.AvatarType,
		AvatarStyle: req.AvatarStyle,
		AvatarURL:   req.AvatarURL,
		AvatarFile:  avatarFile,
	}
	updatedUser, err := h.userService.UpdateProfile(c.Request.Context(), subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromService(updatedUser))
}

func isMultipartRequest(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "multipart/form-data")
}

func bindMultipartProfileRequest(c *gin.Context, req *UpdateProfileRequest) {
	req.Username = optionalFormValue(c, "username")
	req.AvatarType = optionalFormValue(c, "avatar_type")
	req.AvatarStyle = optionalFormValue(c, "avatar_style")
	req.AvatarURL = optionalFormValue(c, "avatar_url")
}

func optionalFormValue(c *gin.Context, key string) *string {
	if _, ok := c.Request.PostForm[key]; !ok {
		return nil
	}
	value := c.PostForm(key)
	return &value
}

func avatarUploadFromForm(form *multipart.Form) (*service.AvatarUpload, error) {
	if form == nil || form.File == nil {
		return nil, nil
	}
	files := form.File["avatar_file"]
	if len(files) == 0 || files[0] == nil {
		return nil, nil
	}

	header := files[0]
	if header.Size > service.MaxAvatarUploadSize {
		return nil, service.ErrAvatarFileTooLarge
	}
	file, err := header.Open()
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, service.MaxAvatarUploadSize+1))
	_ = file.Close()
	if err != nil {
		return nil, err
	}
	if len(data) > service.MaxAvatarUploadSize {
		return nil, service.ErrAvatarFileTooLarge
	}
	return &service.AvatarUpload{
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Reader:      bytes.NewReader(data),
		Size:        int64(len(data)),
	}, nil
}
