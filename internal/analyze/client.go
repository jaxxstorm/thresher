package analyze

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type EndpointStyle string

const (
	EndpointAuto            EndpointStyle = "auto"
	EndpointMessages        EndpointStyle = "messages"
	EndpointChatCompletions EndpointStyle = "chat-completions"
	EndpointResponses       EndpointStyle = "responses"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	style      EndpointStyle
}

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

type AnalyzeRequest struct {
	Model       string
	System      string
	Prompt      string
	MaxTokens   int
	Temperature float64
}

type AnalyzeResponse struct {
	Text string
}

type ModelInfo struct {
	ID string `json:"id"`
}

func NewClient(baseURL string, style EndpointStyle) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		style:      style,
		httpClient: defaultHTTPClient,
	}
}

func (c *Client) Analyze(ctx context.Context, req AnalyzeRequest) (AnalyzeResponse, error) {
	style := c.style
	if style == "" {
		style = EndpointAuto
	}
	if style == EndpointAuto {
		style = EndpointChatCompletions
	}

	var path string
	var payload any
	var decode func([]byte) (AnalyzeResponse, error)

	switch style {
	case EndpointMessages:
		path = "/v1/messages"
		payload = map[string]any{
			"model":      req.Model,
			"max_tokens": req.MaxTokens,
			"messages": []map[string]string{{
				"role":    "user",
				"content": buildPrompt(req.System, req.Prompt),
			}},
		}
		decode = decodeMessagesResponse
	case EndpointResponses:
		path = "/v1/responses"
		payload = map[string]any{
			"model": req.Model,
			"input": []map[string]string{{
				"role":    "user",
				"content": buildPrompt(req.System, req.Prompt),
			}},
		}
		decode = decodeResponsesResponse
	default:
		path = "/v1/chat/completions"
		payload = map[string]any{
			"model": req.Model,
			"messages": []map[string]string{{
				"role":    "user",
				"content": buildPrompt(req.System, req.Prompt),
			}},
		}
		decode = decodeChatResponse
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return AnalyzeResponse{}, fmt.Errorf("marshaling analysis request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return AnalyzeResponse{}, fmt.Errorf("building analysis request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return AnalyzeResponse{}, fmt.Errorf("sending analysis request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return AnalyzeResponse{}, fmt.Errorf("reading analysis response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return AnalyzeResponse{}, fmt.Errorf("analysis request failed: %s", strings.TrimSpace(string(respBody)))
	}

	return decode(respBody)
}

func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("building model list request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("requesting model list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("model discovery unavailable: %s", resp.Status)
	}

	var payload struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding model list: %w", err)
	}
	return payload.Data, nil
}

func buildPrompt(system, prompt string) string {
	if strings.TrimSpace(system) == "" {
		return prompt
	}
	return system + "\n\n" + prompt
}

func decodeChatResponse(body []byte) (AnalyzeResponse, error) {
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return AnalyzeResponse{}, fmt.Errorf("decoding chat response: %w", err)
	}
	if len(payload.Choices) == 0 {
		return AnalyzeResponse{}, fmt.Errorf("chat response missing choices")
	}
	return AnalyzeResponse{Text: payload.Choices[0].Message.Content}, nil
}

func decodeMessagesResponse(body []byte) (AnalyzeResponse, error) {
	var payload struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return AnalyzeResponse{}, fmt.Errorf("decoding messages response: %w", err)
	}
	if len(payload.Content) == 0 {
		return AnalyzeResponse{}, fmt.Errorf("messages response missing content")
	}
	return AnalyzeResponse{Text: payload.Content[0].Text}, nil
}

func decodeResponsesResponse(body []byte) (AnalyzeResponse, error) {
	var payload struct {
		Output []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return AnalyzeResponse{}, fmt.Errorf("decoding responses response: %w", err)
	}
	if len(payload.Output) == 0 || len(payload.Output[0].Content) == 0 {
		return AnalyzeResponse{}, fmt.Errorf("responses payload missing content")
	}
	return AnalyzeResponse{Text: payload.Output[0].Content[0].Text}, nil
}
