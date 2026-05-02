package admin

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

type createPluginRequest struct {
	Name        string                         `json:"name" binding:"required"`
	Type        service.PluginType             `json:"type" binding:"required"`
	Description string                         `json:"description"`
	BaseURL     string                         `json:"base_url"`
	APIKey      string                         `json:"api_key"`
	Enabled     bool                           `json:"enabled"`
	APIPrompt   *service.APIPromptPluginConfig `json:"api_prompt,omitempty"`
}

type updatePluginRequest struct {
	Description *string                        `json:"description,omitempty"`
	BaseURL     *string                        `json:"base_url,omitempty"`
	APIKey      *string                        `json:"api_key,omitempty"`
	Enabled     *bool                          `json:"enabled,omitempty"`
	APIPrompt   *service.APIPromptPluginConfig `json:"api_prompt,omitempty"`
}

type setPluginEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *PluginHandler) List(c *gin.Context) {
	plugins, err := h.pluginService.ListPlugins(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plugins)
}

func (h *PluginHandler) Create(c *gin.Context) {
	var req createPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	plugin, err := h.pluginService.CreatePlugin(c.Request.Context(), service.CreatePluginRequest{
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		BaseURL:     req.BaseURL,
		APIKey:      req.APIKey,
		Enabled:     req.Enabled,
		APIPrompt:   req.APIPrompt,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plugin)
}

func (h *PluginHandler) Update(c *gin.Context) {
	var req updatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	plugin, err := h.pluginService.UpdatePlugin(c.Request.Context(), c.Param("name"), service.UpdatePluginRequest{
		Description: req.Description,
		BaseURL:     req.BaseURL,
		APIKey:      req.APIKey,
		Enabled:     req.Enabled,
		APIPrompt:   req.APIPrompt,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plugin)
}

func (h *PluginHandler) SetEnabled(c *gin.Context) {
	var req setPluginEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	plugin, err := h.pluginService.SetPluginEnabled(c.Request.Context(), c.Param("name"), req.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plugin)
}

func (h *PluginHandler) Test(c *gin.Context) {
	result, err := h.pluginService.TestPlugin(c.Request.Context(), c.Param("name"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
