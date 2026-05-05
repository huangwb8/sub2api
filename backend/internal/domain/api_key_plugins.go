package domain

// APIKeyPluginSettings stores extensible per-key plugin bindings.
type APIKeyPluginSettings struct {
	APIPrompt *APIPromptKeyBinding `json:"api_prompt,omitempty"`
}

// APIPromptKeyBinding binds an API key to a specific api-prompt plugin template.
type APIPromptKeyBinding struct {
	PluginName  string `json:"plugin_name,omitempty"`
	TemplateID  string `json:"template_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	Custom      bool   `json:"custom,omitempty"`
}
