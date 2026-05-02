package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func applyPluginPromptTemplate(
	ctx context.Context,
	pluginService *service.PluginService,
	apiKey *service.APIKey,
	body []byte,
	target service.PluginPromptTarget,
) ([]byte, error) {
	if pluginService == nil || apiKey == nil || len(body) == 0 {
		return body, nil
	}
	updated, _, err := pluginService.ApplyBoundPromptTemplate(ctx, body, target, apiKey.PluginSettings)
	if err != nil {
		return nil, err
	}
	return updated, nil
}
