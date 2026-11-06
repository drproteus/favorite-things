// Package suggest reads a photo of an item and proposes a title, description and hashtags.
// It calls Amazon Bedrock through the Converse API. Nothing is saved; the form shows the suggestion for the user to edit.
package suggest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

const (
	maxTitle       = 200
	maxDescription = 10000
	maxTags        = 10
	maxTagLength   = 40
)

const prompt = `You catalog a personal collection. Look at the photo of one item and reply with only a JSON object, no other text:
{"title": "short product name, e.g. Nintendo Game Boy Color, Blue", "description": "one to three sentences about what it is and its visible condition", "tags": ["up to five lowercase words, no spaces, for category or traits"]}
Describe only what is visible. If you are unsure of the exact model, year or edition, leave that out. If you cannot tell what the item is, use "Unknown item" as the title.`

// Suggestion is what the form is prefilled with. Tags are also written into Description as #hashtags,
// because hashtags in the description are what the app stores as tags.
type Suggestion struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// Model describes an image and returns the model's raw text reply.
type Model interface {
	Describe(ctx context.Context, image []byte, format string) (string, error)
}

// Bedrock is a Model backed by a Bedrock model that accepts images, such as Amazon Nova Lite.
type Bedrock struct {
	client  *bedrockruntime.Client
	modelID string
}

// NewBedrock uses the standard AWS credential chain: AWS_PROFILE or SSO locally, or the
// instance or task role once deployed.
func NewBedrock(ctx context.Context, region, modelID string) (*Bedrock, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return &Bedrock{client: bedrockruntime.NewFromConfig(cfg), modelID: modelID}, nil
}

var bedrockFormats = map[string]types.ImageFormat{
	"jpeg": types.ImageFormatJpeg,
	"png":  types.ImageFormatPng,
	"gif":  types.ImageFormatGif,
	"webp": types.ImageFormatWebp,
}

func (b *Bedrock) Describe(ctx context.Context, image []byte, format string) (string, error) {
	imageFormat, ok := bedrockFormats[format]
	if !ok {
		return "", fmt.Errorf("unsupported image format %q", format)
	}
	out, err := b.client.Converse(ctx, &bedrockruntime.ConverseInput{
		ModelId: aws.String(b.modelID),
		Messages: []types.Message{{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberImage{Value: types.ImageBlock{
					Format: imageFormat,
					Source: &types.ImageSourceMemberBytes{Value: image},
				}},
				&types.ContentBlockMemberText{Value: prompt},
			},
		}},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens:   aws.Int32(400),
			Temperature: aws.Float32(0.2),
		},
	})
	if err != nil {
		return "", err
	}
	msg, ok := out.Output.(*types.ConverseOutputMemberMessage)
	if !ok {
		return "", errors.New("model returned no message")
	}
	var text strings.Builder
	for _, block := range msg.Value.Content {
		if t, ok := block.(*types.ContentBlockMemberText); ok {
			text.WriteString(t.Value)
		}
	}
	return text.String(), nil
}

// parseSuggestion reads the model's JSON reply, tolerating surrounding text or code fences,
// and cleans every field so it fits the app's limits.
func parseSuggestion(raw string) (Suggestion, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return Suggestion{}, errors.New("no JSON object in model reply")
	}
	var s Suggestion
	if err := json.Unmarshal([]byte(raw[start:end+1]), &s); err != nil {
		return Suggestion{}, fmt.Errorf("parse model reply: %w", err)
	}

	title := strings.Join(strings.Fields(s.Title), " ")
	if title == "" {
		title = "Unknown item"
	}
	s.Title = truncate(title, maxTitle)

	var tags []string
	seen := map[string]bool{}
	for _, t := range s.Tags {
		tag := cleanTag(t)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
		if len(tags) == maxTags {
			break
		}
	}
	s.Tags = tags

	// Make sure every tag appears in the description, since that is where tags are read from.
	desc := strings.TrimSpace(s.Description)
	for _, tag := range tags {
		if !strings.Contains(strings.ToLower(desc), "#"+tag) {
			desc = strings.TrimSpace(desc + " #" + tag)
		}
	}
	s.Description = truncate(desc, maxDescription)
	return s, nil
}

// cleanTag turns a model tag such as "Vintage Games" or "#mint" into a hashtag word: "vintage_games", "mint".
func cleanTag(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "#"))) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '-':
			b.WriteRune('_')
		}
	}
	return truncate(strings.Trim(b.String(), "_"), maxTagLength)
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
