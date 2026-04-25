package codex

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type Adaptor struct {
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {

}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode != relayconstant.RelayModeResponses && info.RelayMode != relayconstant.RelayModeResponsesCompact && info.RelayMode != relayconstant.RelayModeChatCompletions && info.RelayMode != relayconstant.RelayModeCompletions {
		return "", errors.New("codex channel: only /v1/responses, /v1/responses/compact and /v1/chat/completions are supported")
	}
	path := "/backend-api/codex/responses"
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		path = "/backend-api/codex/responses/compact"
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, path, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	key := strings.TrimSpace(info.ApiKey)
	if !strings.HasPrefix(key, "{") {
		return errors.New("codex channel: key must be a JSON object")
	}

	oauthKey, err := ParseOAuthKey(key)
	if err != nil {
		return err
	}

	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)

	if accessToken == "" {
		return errors.New("codex channel: access_token is required")
	}
	if accountID == "" {
		return errors.New("codex channel: account_id is required")
	}

	req.Set("Authorization", "Bearer "+accessToken)
	req.Set("chatgpt-account-id", accountID)

	if req.Get("OpenAI-Beta") == "" {
		req.Set("OpenAI-Beta", "responses=experimental")
	}
	if req.Get("originator") == "" {
		req.Set("originator", "codex_cli_rs")
	}

	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else if req.Get("Accept") == "" {
		req.Set("Accept", "application/json")
	}

	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("codex channel: request is nil")
	}

	responsesReq := &dto.OpenAIResponsesRequest{
		Model:  request.Model,
		Stream: lo.ToPtr(true),
	}
	common.SysLog(fmt.Sprintf("Codex request: model=%s, stream=%v, tools=%d", responsesReq.Model, *responsesReq.Stream, len(request.Tools)))

	maxTokens := request.GetMaxTokens()
	if maxTokens > 0 {
		responsesReq.MaxOutputTokens = lo.ToPtr(maxTokens)
	}
	if request.Temperature != nil {
		responsesReq.Temperature = request.Temperature
	}
	if request.TopP != nil {
		responsesReq.TopP = request.TopP
	}

	// Forward Tools and ToolChoice to upstream (convert to flat format)
	if len(request.Tools) > 0 {
		var codexTools []map[string]any
		for _, tool := range request.Tools {
			flatTool := map[string]any{
				"type":        tool.Type,
				"name":        tool.Function.Name,
				"description": tool.Function.Description,
				"parameters":  tool.Function.Parameters,
			}
			codexTools = append(codexTools, flatTool)
		}
		if b, err := json.Marshal(codexTools); err == nil {
			responsesReq.Tools = b
		}
	}
	if request.ToolChoice != nil {
		if b, err := json.Marshal(request.ToolChoice); err == nil {
			responsesReq.ToolChoice = b
		}
	}

	// Convert messages or prompt to input
	var instructions []string
	if len(request.Messages) > 0 {
		var codexInput []map[string]any
		for _, msg := range request.Messages {
			content := msg.StringContent()
			if msg.Role == "system" {
				instructions = append(instructions, content)
				continue
			}

			role := msg.Role
			contentType := "input_text"
			if role == "assistant" {
				contentType = "output_text"
				// Handle tool_calls in history
				if content == "" && len(msg.ToolCalls) > 0 {
					b, _ := json.Marshal(msg.ToolCalls)
					content = string(b)
				}
			} else if role == "tool" {
				// Codex doesn't have a tool role, wrap it in user message
				role = "user"
				content = fmt.Sprintf("Tool result [%s]: %s", msg.ToolCallId, content)
			}

			if content == "" {
				continue
			}

			codexMsg := map[string]any{
				"role": role,
				"content": []map[string]any{
					{
						"type": contentType,
						"text": content,
					},
				},
			}
			codexInput = append(codexInput, codexMsg)
		}
		if len(codexInput) > 0 {
			input, err := json.Marshal(codexInput)
			if err != nil {
				return nil, err
			}
			responsesReq.Input = input
		}
		if len(instructions) > 0 {
			if b, err := json.Marshal(strings.Join(instructions, "\n")); err == nil {
				responsesReq.Instructions = b
			}
		}
	} else if request.Prompt != nil {
		promptStr := ""
		switch v := request.Prompt.(type) {
		case string:
			promptStr = v
		case []any:
			if len(v) > 0 {
				if s, ok := v[0].(string); ok {
					promptStr = s
				}
			}
		}
		if promptStr != "" {
			codexInput := []map[string]any{
				{
					"role": "user",
					"content": []map[string]any{
						{
							"type": "input_text",
							"text": promptStr,
						},
					},
				},
			}
			input, err := json.Marshal(codexInput)
			if err != nil {
				return nil, err
			}
			responsesReq.Input = input
		}
	}

	return a.ConvertOpenAIResponsesRequest(c, info, *responsesReq)
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// Upstream Codex ALWAYS requires stream=true
	request.Stream = lo.ToPtr(true)

	// Fix input format
	if len(request.Input) > 0 {
		var inputData any
		if err := json.Unmarshal(request.Input, &inputData); err == nil {
			var codexInput []map[string]any
			switch v := inputData.(type) {
			case string:
				codexInput = append(codexInput, map[string]any{
					"role": "user",
					"content": []map[string]any{
						{
							"type": "input_text",
							"text": v,
						},
					},
				})
			case []any:
				if len(v) > 0 {
					if _, isStr := v[0].(string); isStr {
						for _, item := range v {
							codexInput = append(codexInput, map[string]any{
								"role": "user",
								"content": []map[string]any{
									{
										"type": "input_text",
										"text": item.(string),
									},
								},
							})
						}
					}
				}
			}
			if codexInput != nil {
				if newBuf, err := json.Marshal(codexInput); err == nil {
					request.Input = newBuf
				}
			}
		}
	}

	common.SysLog(fmt.Sprintf("Final Codex Input: %s", string(request.Input)))

	if len(request.Instructions) == 0 {
		systemPrompt := "You are a helpful assistant."
		if info != nil && info.ChannelSetting.SystemPrompt != "" {
			systemPrompt = info.ChannelSetting.SystemPrompt
		}
		if b, err := common.Marshal(systemPrompt); err == nil {
			request.Instructions = b
		}
	}

	// codex: store must be false
	request.Store = json.RawMessage("false")
	request.MaxOutputTokens = nil
	request.Temperature = nil
	return request, nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertRealtimeRequest(c *gin.Context, info *relaycommon.RelayInfo, request any) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode != relayconstant.RelayModeResponses && info.RelayMode != relayconstant.RelayModeResponsesCompact && info.RelayMode != relayconstant.RelayModeChatCompletions && info.RelayMode != relayconstant.RelayModeCompletions {
		return nil, types.NewError(errors.New("codex channel: endpoint not supported"), types.ErrorCodeInvalidRequest)
	}

	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		return openai.OaiResponsesCompactionHandler(c, resp)
	}

	isChatPath := strings.Contains(c.Request.URL.Path, "/chat/")
	isCompletionPath := strings.Contains(c.Request.URL.Path, "/completions") && !isChatPath

	// If client wants stream
	if info.IsStream {
		if info.RelayMode == relayconstant.RelayModeChatCompletions || info.RelayMode == relayconstant.RelayModeCompletions || isChatPath || isCompletionPath {
			return openai.OaiResponsesToChatStreamHandler(c, info, resp)
		}
		return openai.OaiResponsesStreamHandler(c, info, resp)
	}

	// If client wants non-stream, we must buffer and combine the stream
	defer service.CloseResponseBodyGracefully(resp)

	var responseTextBuilder strings.Builder
	var lastUsage *dto.Usage
	var lastResponse *dto.OpenAIResponsesResponse

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var streamResponse dto.ResponsesStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResponse); err != nil {
			continue
		}

		switch streamResponse.Type {
		case "response.output_text.delta":
			responseTextBuilder.WriteString(streamResponse.Delta)
		case "response.completed":
			if streamResponse.Response != nil {
				lastResponse = streamResponse.Response
				if streamResponse.Response.Usage != nil {
					lastUsage = &dto.Usage{
						PromptTokens:     streamResponse.Response.Usage.InputTokens,
						CompletionTokens: streamResponse.Response.Usage.OutputTokens,
						TotalTokens:      streamResponse.Response.Usage.TotalTokens,
					}
				}
			}
		}
	}

	if lastResponse == nil {
		return nil, types.NewError(errors.New("upstream failed to provide final response"), types.ErrorCodeBadResponse)
	}

	fullText := responseTextBuilder.String()
	common.SysLog(fmt.Sprintf("Combined response [Path: %s, Mode: %d]: %d chars", c.Request.URL.Path, info.RelayMode, len(fullText)))

	// Convert to standard OpenAI format for Chat and Completions paths
	var finalResponse any
	if isChatPath || info.RelayMode == relayconstant.RelayModeChatCompletions {
		finalResponse = gin.H{
			"id":      lastResponse.ID,
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   info.OriginModelName,
			"choices": []gin.H{
				{
					"index": 0,
					"message": gin.H{
						"role":    "assistant",
						"content": fullText,
					},
					"finish_reason": "stop",
				},
			},
			"usage": lastUsage,
		}
	} else if isCompletionPath || info.RelayMode == relayconstant.RelayModeCompletions {
		finalResponse = gin.H{
			"id":      lastResponse.ID,
			"object":  "text_completion",
			"created": time.Now().Unix(),
			"model":   info.OriginModelName,
			"choices": []gin.H{
				{
					"text":          fullText,
					"index":         0,
					"finish_reason": "stop",
				},
			},
			"usage": lastUsage,
		}
	} else {
		// Native Responses format
		if len(lastResponse.Output) == 0 {
			lastResponse.Output = []dto.ResponsesOutput{
				{
					Type:   "message",
					ID:     "msg_" + lastResponse.ID,
					Status: "completed",
					Role:   "assistant",
					Content: []dto.ResponsesOutputContent{
						{
							Type: "text",
							Text: fullText,
						},
					},
				},
			}
		}
		finalResponse = lastResponse
	}

	// Use AbortWithStatusJSON to ensure nothing else touches the response
	c.Writer.Header().Del("Transfer-Encoding")
	c.AbortWithStatusJSON(http.StatusOK, finalResponse)

	if lastUsage == nil {
		lastUsage = &dto.Usage{}
	}
	return lastUsage, nil
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesCompactionRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.OpenAIResponsesCompactionRequest) (any, error) {
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertToOpenAIAudio(originTask *model.Task) ([]byte, error) {
	return nil, errors.New("not supported")
}
