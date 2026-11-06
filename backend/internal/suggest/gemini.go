package suggest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// Gemini is a Model backed by the Gemini API. It needs only an API key from Google AI Studio.
type Gemini struct {
	client  *http.Client
	apiKey  string
	model   string
	baseURL string
}

func NewGemini(apiKey, model string) *Gemini {
	return &Gemini{
		client:  &http.Client{Timeout: 60 * time.Second},
		apiKey:  apiKey,
		model:   model,
		baseURL: geminiBaseURL,
	}
}

type geminiPart struct {
	Text       string        `json:"text,omitempty"`
	InlineData *geminiInline `json:"inline_data,omitempty"`
}

type geminiInline struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiRequest struct {
	Contents []struct {
		Parts []geminiPart `json:"parts"`
	} `json:"contents"`
	GenerationConfig struct {
		Temperature      float64 `json:"temperature"`
		MaxOutputTokens  int     `json:"maxOutputTokens"`
		ResponseMIMEType string  `json:"responseMimeType"`
	} `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (g *Gemini) Describe(ctx context.Context, image []byte, format string) (string, error) {
	var req geminiRequest
	req.Contents = []struct {
		Parts []geminiPart `json:"parts"`
	}{{Parts: []geminiPart{
		{InlineData: &geminiInline{MimeType: "image/" + format, Data: base64.StdEncoding.EncodeToString(image)}},
		{Text: prompt},
	}}}
	req.GenerationConfig.Temperature = 0.2
	// Gemini 3 models spend part of this budget on internal reasoning, so leave room for the JSON.
	req.GenerationConfig.MaxOutputTokens = 2048
	req.GenerationConfig.ResponseMIMEType = "application/json"

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	endpoint := fmt.Sprintf("%s/models/%s:generateContent", g.baseURL, url.PathEscape(g.model))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", g.apiKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var out geminiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("gemini returned status %d and an unreadable body", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK || out.Error != nil {
		msg := "unknown error"
		if out.Error != nil {
			msg = out.Error.Status + ": " + out.Error.Message
		}
		return "", fmt.Errorf("gemini returned status %d: %s", resp.StatusCode, msg)
	}
	if len(out.Candidates) == 0 {
		return "", errors.New("gemini returned no candidates")
	}
	if reason := out.Candidates[0].FinishReason; reason != "" && reason != "STOP" {
		return "", fmt.Errorf("gemini stopped before finishing: %s", reason)
	}
	var text strings.Builder
	for _, p := range out.Candidates[0].Content.Parts {
		text.WriteString(p.Text)
	}
	return text.String(), nil
}
