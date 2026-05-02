package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PluginHandler struct {
	pluginService *service.PluginService
}

func NewPluginHandler(pluginService *service.PluginService) *PluginHandler {
	return &PluginHandler{pluginService: pluginService}
}

func (h *PluginHandler) ListAPIPromptTemplates(c *gin.Context) {
	templates, err := h.pluginService.ListAPIPromptTemplateOptions(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, templates)
}
