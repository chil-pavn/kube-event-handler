package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	openrouter "github.com/wojtess/openrouter-api-go"
)

// AIAnalysisResult represents the structured response from AI analysis
type AIAnalysisResult struct {
	Description string   `json:"description"`
	Details     []string `json:"details"`
	Solutions   []string `json:"solutions"`
}

// AIAnalyzer is an interface for AI analysis
type AIAnalyzer interface {
	Analyze(logs string) (*AIAnalysisResult, error)
}

// OpenAIAnalyzer implements AIAnalyzer for OpenAI
type OpenAIAnalyzer struct {
	client *openai.Client
}

func NewOpenAIAnalyzer(apiKey string) *OpenAIAnalyzer {
	return &OpenAIAnalyzer{
		client: openai.NewClient(apiKey),
	}
}

func (a *OpenAIAnalyzer) Analyze(logs string) (*AIAnalysisResult, error) {
	DebugLog("Starting OpenAI analysis", "logs", logs)

	prompt := `Please analyze these Kubernetes logs and provide a detailed analysis. 
	Format your response as a valid JSON object with the following structure:
	{
		"description": "A clear, detailed description of the issue",
		"details": [
			"Specific error messages or relevant log entries",
			"System state information",
			"Affected components"
		],
		"solutions": [
			"Step-by-step remediation steps",
			"Long-term preventive measures",
			"Additional monitoring suggestions"
		]
	}`

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4oMini20240718,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: prompt,
			},
			{
				Role:    "user",
				Content: logs,
			},
		},
	}

	resp, err := a.client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		DebugLog("OpenAI analysis failed", "error", err)
		return nil, fmt.Errorf("failed to create chat completion: %v", err)
	}

	if len(resp.Choices) == 0 {
		DebugLog("OpenAI analysis returned no choices")
		return nil, fmt.Errorf("no choices returned in response")
	}

	var analysisResult AIAnalysisResult
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &analysisResult); err != nil {
		DebugLog("Failed to parse OpenAI response content", "error", err, "content", resp.Choices[0].Message.Content)
		return nil, fmt.Errorf("failed to parse AI response content: %v, content: %s", err, resp.Choices[0].Message.Content)
	}

	DebugLog("OpenAI analysis completed", "result", analysisResult)
	return &analysisResult, nil
}

// OpenRouterAnalyzer implements AIAnalyzer for OpenRouter
type OpenRouterAnalyzer struct {
	client *openrouter.OpenRouterClient
}

func NewOpenRouterAnalyzer(apiKey string) *OpenRouterAnalyzer {
	client := openrouter.NewOpenRouterClient(apiKey)
	return &OpenRouterAnalyzer{
		client: client,
	}
}

func (a *OpenRouterAnalyzer) Analyze(logs string) (*AIAnalysisResult, error) {
	DebugLog("Starting OpenRouter analysis", "logs", logs)

	prompt := `Please analyze these Kubernetes logs and provide a detailed analysis. 
	Format your response as a valid JSON object with the following structure:
	{
		"description": "A clear, detailed description of the issue",
		"details": [
			"Specific error messages or relevant log entries",
			"System state information",
			"Affected components"
		],
		"solutions": [
			"Step-by-step remediation steps",
			"Long-term preventive measures",
			"Additional monitoring suggestions"
		]
	}`

	request := openrouter.Request{
		Model: "deepseek/deepseek-r1:free",
		Messages: []openrouter.MessageRequest{
			{Role: openrouter.RoleUser, Content: prompt + "\n\n" + logs},
		},
	}

	response, err := a.client.FetchChatCompletions(request)
	if err != nil {
		DebugLog("OpenRouter analysis failed", "error", err)
		return nil, fmt.Errorf("failed to create chat completion: %v", err)
	}

	DebugLog("OpenRouter response received", "response", response)

	if len(response.Choices) == 0 {
		DebugLog("OpenRouter analysis returned no choices")
		return nil, fmt.Errorf("no choices returned in response")
	}

	var analysisResult AIAnalysisResult
	if err := json.Unmarshal([]byte(response.Choices[0].Message.Content), &analysisResult); err != nil {
		DebugLog("Failed to parse OpenRouter response content", "error", err, "content", response.Choices[0].Message.Content)
		return nil, fmt.Errorf("failed to parse AI response content: %v, content: %s", err, response.Choices[0].Message.Content)
	}

	DebugLog("OpenRouter analysis completed", "result", analysisResult)
	return &analysisResult, nil
}

// SlackMessage represents a formatted Slack message
type SlackMessage struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments"`
}

// Attachment represents a Slack message attachment
type Attachment struct {
	Color  string  `json:"color"`
	Fields []Field `json:"fields"`
	Text   string  `json:"text"`
}

// Field represents a field in a Slack attachment
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// OpenAIResponse represents the complete response from OpenAI API
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnalyzeFailure takes failure logs and returns AI analysis
func AnalyzeFailure(logs string) (*AIAnalysisResult, error) {
	DebugLog("Starting failure analysis", "logs", logs)

	var analyzer AIAnalyzer

	openaiKey := os.Getenv("OPENAI_API_KEY")
	openrtKey := os.Getenv("OPENRT_API_KEY")
	provider := os.Getenv("PROVIDER")

	if openaiKey != "" && (provider == "" || provider == "OPENAI") {
		analyzer = NewOpenAIAnalyzer(openaiKey)
	} else if openrtKey != "" && (provider == "" || provider == "OPENRT") {
		analyzer = NewOpenRouterAnalyzer(openrtKey)
	} else {
		DebugLog("No valid AI provider configuration found")
		return nil, fmt.Errorf("no valid AI provider configuration found")
	}

	result, err := analyzer.Analyze(logs)
	if err != nil {
		DebugLog("Failure analysis failed", "error", err)
		return nil, err
	}

	DebugLog("Failure analysis completed", "result", result)
	return result, nil
}

// SendToSlack sends the analysis results to a Slack channel
func SendToSlack(analysis *AIAnalysisResult) error {
	DebugLog("Sending analysis results to Slack", "analysis", analysis)

	webhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	if webhookURL == "" {
		DebugLog("SLACK_WEBHOOK_URL environment variable is not set")
		return fmt.Errorf("SLACK_WEBHOOK_URL environment variable is not set")
	}

	message := SlackMessage{
		Text: "🚨 Kubernetes Failure Analysis",
		Attachments: []Attachment{
			{
				Color: "#ff0000",
				Fields: []Field{
					{
						Title: "Issue Description",
						Value: analysis.Description,
						Short: false,
					},
					{
						Title: "Key Details",
						Value: strings.Join(analysis.Details, "\n"),
						Short: false,
					},
					{
						Title: "Suggested Solutions",
						Value: strings.Join(analysis.Solutions, "\n"),
						Short: false,
					},
				},
			},
		},
	}

	jsonMessage, err := json.Marshal(message)
	if err != nil {
		DebugLog("Failed to marshal Slack message", "error", err)
		return fmt.Errorf("failed to marshal slack message: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonMessage))
	if err != nil {
		DebugLog("Failed to send Slack message", "error", err)
		return fmt.Errorf("failed to send slack message: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		DebugLog("Slack API returned non-200 status code", "status_code", resp.StatusCode)
		return fmt.Errorf("slack API returned non-200 status code: %d", resp.StatusCode)
	}

	DebugLog("Slack message sent successfully")
	return nil
}

// ProcessFailure handles the complete workflow of analyzing and reporting a failure
func ProcessFailure(logs string) error {
	DebugLog("Starting failure processing", "logs", logs)

	// Check if required environment variables are set
	if os.Getenv("SLACK_WEBHOOK_URL") == "" {
		DebugLog("SLACK_WEBHOOK_URL environment variable is not set")
		return fmt.Errorf("SLACK_WEBHOOK_URL environment variable is not set")
	}

	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("OPENRT_API_KEY") == "" {
		DebugLog("AI provider API key environment variable is not set")
		return fmt.Errorf("AI provider API key environment variable is not set")
	}

	// Analyze the failure using AI
	analysis, err := AnalyzeFailure(logs)
	if err != nil {
		DebugLog("Analysis failed", "error", err)
		return fmt.Errorf("failed to analyze failure: %v", err)
	}

	DebugLog("Analysis completed", "result", analysis)

	// Send results to Slack
	if err := SendToSlack(analysis); err != nil {
		DebugLog("Slack notification failed", "error", err)
		return fmt.Errorf("failed to send to slack: %v", err)
	}

	DebugLog("Failure processing completed successfully")
	return nil
}
