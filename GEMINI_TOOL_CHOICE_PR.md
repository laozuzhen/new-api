# PR: feat(gemini): support tool_choice parameter and improve error handling

## 问题描述 / Problem

### 问题 1: tool_choice 参数不生效
当使用 OpenAI 兼容格式调用 Gemini API 时，`tool_choice` 参数没有被转换为 Gemini 的 `toolConfig.functionCallingConfig`，导致：
- `tool_choice: "required"` 不生效，AI 可能返回纯文本而不是调用工具
- `tool_choice: "none"` 不生效，AI 仍然可能调用工具

### 问题 2: FinishReason 处理不完整
Gemini API 返回的 FinishReason 有多种值（SAFETY, RECITATION, BLOCKLIST, PROHIBITED_CONTENT, SPII, OTHER），但目前除了 STOP 和 MAX_TOKENS 外，其他都被简单地转换为 content_filter，没有明确的注释说明。

### 问题 3: Candidates 为空时返回空响应
当 Gemini 因为安全过滤等原因返回空 Candidates 时，代码被注释掉了，导致返回空响应而不是有意义的错误信息。

## 解决方案 / Solution

### 1. tool_choice 转换
在 `relay/channel/gemini/relay-gemini.go` 中添加 `tool_choice` 到 `toolConfig.functionCallingConfig` 的转换：

| OpenAI tool_choice | Gemini mode |
|-------------------|-------------|
| `"auto"` | `"AUTO"` |
| `"none"` | `"NONE"` |
| `"required"` | `"ANY"` |
| `{"type": "function", "function": {"name": "xxx"}}` | `"ANY"` + `allowedFunctionNames: ["xxx"]` |

### 2. FinishReason 完整映射
添加所有 Gemini 特有的 FinishReason 值的处理：
- SAFETY - 安全过滤
- RECITATION - 引用检测
- BLOCKLIST - 黑名单
- PROHIBITED_CONTENT - 禁止内容
- SPII - 敏感个人信息
- OTHER - 其他原因

### 3. 空 Candidates 错误处理
取消注释并启用空 Candidates 的错误处理：
- 如果有 `PromptFeedback.BlockReason`，返回具体的阻止原因
- 否则返回 "empty response from Gemini API" 错误

## 测试 / Testing

### 测试用例 1: tool_choice: "required"
```bash
curl -X POST "http://localhost:3000/v1/chat/completions" \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash",
    "messages": [{"role": "user", "content": "What is the weather?"}],
    "tools": [{"type": "function", "function": {"name": "get_weather", "parameters": {"type": "object", "properties": {"location": {"type": "string"}}}}}],
    "tool_choice": "required"
  }'
```
预期：AI 必须返回 `tool_calls`，不能返回纯文本。

### 测试用例 2: tool_choice: "none"
预期：AI 返回纯文本，不调用工具。

## 参考文档 / References
- [Gemini API - Function Calling Config](https://ai.google.dev/gemini-api/docs/function-calling#function_calling_mode)
- [OpenAI API - tool_choice](https://platform.openai.com/docs/api-reference/chat/create#chat-create-tool_choice)

## 修改的文件 / Changed Files
- `relay/channel/gemini/relay-gemini.go`
