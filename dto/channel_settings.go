package dto

type ChannelSettings struct {
	ForceFormat            bool   `json:"force_format,omitempty"`
	ThinkingToContent      bool   `json:"thinking_to_content,omitempty"`
	Proxy                  string `json:"proxy"`
	PassThroughBodyEnabled bool   `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt           string `json:"system_prompt,omitempty"`
	SystemPromptOverride   bool   `json:"system_prompt_override,omitempty"`
	// 渠道级别速率限制
	RateLimitRPM           int    `json:"rate_limit_rpm,omitempty"`           // 每分钟请求数限制，0 表示不限制
	RateLimitRPD           int    `json:"rate_limit_rpd,omitempty"`           // 每天请求数限制，0 表示不限制
	RateLimitEnabled       bool   `json:"rate_limit_enabled,omitempty"`       // 是否启用速率限制
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type AwsKeyType string

const (
	AwsKeyTypeAKSK   AwsKeyType = "ak_sk" // 默认
	AwsKeyTypeApiKey AwsKeyType = "api_key"
)

type ChannelOtherSettings struct {
	AzureResponsesVersion string        `json:"azure_responses_version,omitempty"`
	VertexKeyType         VertexKeyType `json:"vertex_key_type,omitempty"` // "json" or "api_key"
	OpenRouterEnterprise  *bool         `json:"openrouter_enterprise,omitempty"`
	AllowServiceTier      bool          `json:"allow_service_tier,omitempty"`      // 是否允许 service_tier 透传（默认过滤以避免额外计费）
	DisableStore          bool          `json:"disable_store,omitempty"`           // 是否禁用 store 透传（默认允许透传，禁用后可能导致 Codex 无法使用）
	AllowSafetyIdentifier bool          `json:"allow_safety_identifier,omitempty"` // 是否允许 safety_identifier 透传（默认过滤以保护用户隐私）
	AwsKeyType            AwsKeyType    `json:"aws_key_type,omitempty"`
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}
