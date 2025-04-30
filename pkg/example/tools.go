package example

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/spf13/viper"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewHelloTool returns the hello tool and its handler
func NewHelloTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("hello_world",
		mcp.WithDescription("Say hello to someone"),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "Say hello",
			ReadOnlyHint: true,
		}),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the person to greet"),
		),
	)
	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, ok := request.Params.Arguments["name"].(string)
		if !ok || name == "" {
			return mcp.NewToolResultError("name must be a string"), nil
		}
		greeting := viper.GetString("greeting")
		if greeting == "" {
			greeting = "Hello"
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s, %s!", greeting, name)), nil
	}
	return tool, handler
}

// NewEnumTool returns a tool that demonstrates enum usage
func NewEnumTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("choose_color",
		mcp.WithDescription("Choose a color from a predefined set of options"),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "Choose a color",
			ReadOnlyHint: true,
		}),
		mcp.WithString("color",
			mcp.Required(),
			mcp.Description("The color to choose"),
			mcp.Enum("red", "green", "blue"),
		),
	)
	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		color, ok := request.Params.Arguments["color"].(string)
		if !ok || color == "" {
			return mcp.NewToolResultError("color must be one of: red, green, blue"), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("You chose the color: %s", color)), nil
	}
	return tool, handler
}

// NewFetchWebpageTool returns a tool that fetches content from a webpage
func NewFetchWebpageTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("fetch_webpage",
		mcp.WithDescription("Fetches content from a webpage"),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "Fetch Webpage",
			ReadOnlyHint: true,
		}),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL of the webpage to fetch"),
		),
	)
	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url, ok := request.Params.Arguments["url"].(string)
		if !ok || url == "" {
			return mcp.NewToolResultError("url must be a string"), nil
		}

		client := &http.Client{}
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create request: %v", err)), nil
		}

		resp, err := client.Do(req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to fetch webpage: %v", err)), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return mcp.NewToolResultError(fmt.Sprintf("unexpected status code: %d", resp.StatusCode)), nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read response body: %v", err)), nil
		}

		return mcp.NewToolResultText(string(body)), nil
	}
	return tool, handler
}

// NewTextToSpeechTool returns a tool that converts text to speech
func NewTextToSpeechTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("text_to_speech",
		mcp.WithDescription("Convert text to speech using OpenAI"),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "Text to Speech",
			ReadOnlyHint: false,
		}),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("The text to convert to speech"),
		),
		mcp.WithString("voice",
			mcp.Description("The OpenAI voice to use"),
			mcp.Enum("alloy", "echo", "fable", "onyx", "nova", "shimmer"),
		),
		mcp.WithString("outputFile",
			mcp.Required(),
			mcp.Description("The path where to save the MP3 file"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, ok := request.Params.Arguments["text"].(string)
		if !ok || text == "" {
			return mcp.NewToolResultError("text must be a non-empty string"), nil
		}

		outputFile, ok := request.Params.Arguments["outputFile"].(string)
		if !ok || outputFile == "" {
			return mcp.NewToolResultError("outputFile must be a non-empty string"), nil
		}

		voice := "alloy" // default voice
		if voiceArg, ok := request.Params.Arguments["voice"].(string); ok && voiceArg != "" {
			voice = voiceArg
		}

		// Get OpenAI API key from environment
		apiKey := viper.GetString("OPEN_API_KEY")
		if apiKey == "" {
			return mcp.NewToolResultError("OPEN_API_KEY environment variable is not set"), nil
		}

		// Create the OpenAI request
		client := &http.Client{}
		openaiRequest := map[string]interface{}{
			"model": "tts-1",
			"input": text,
			"voice": voice,
		}

		jsonData, err := json.Marshal(openaiRequest)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create request: %v", err)), nil
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/speech", bytes.NewBuffer(jsonData))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create request: %v", err)), nil
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

		resp, err := client.Do(req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to call OpenAI: %v", err)), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return mcp.NewToolResultError(fmt.Sprintf("OpenAI returned unexpected status code: %d\nResponse: %s", resp.StatusCode, string(body))), nil
		}

		// Read the audio data from the response
		audioData, err := io.ReadAll(resp.Body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read audio data: %v", err)), nil
		}

		// Save the audio data to the specified file
		if err := os.WriteFile(outputFile, audioData, 0644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save audio file: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Successfully saved audio to %s", outputFile)), nil
	}
	return tool, handler
}

// NewMacSayTool returns a tool that uses macOS's say command for text-to-speech
func NewMacSayTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("mac_say",
		mcp.WithDescription("Convert text to speech using macOS say command"),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "macOS Text to Speech",
			ReadOnlyHint: false,
		}),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("The text to convert to speech"),
		),
		mcp.WithString("voice",
			mcp.Description("The voice to use (optional). Use system voices like 'Alex', 'Victoria', etc."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, ok := request.Params.Arguments["text"].(string)
		if !ok || text == "" {
			return mcp.NewToolResultError("text must be a non-empty string"), nil
		}

		command := "say"
		if voice, ok := request.Params.Arguments["voice"].(string); ok && voice != "" {
			command = fmt.Sprintf("say -v %s", voice)
		}

		// Create a temporary shell script to run the command
		scriptContent := fmt.Sprintf("%s -v Moira %q", command, text)
		cmd := exec.CommandContext(ctx, "sh", "-c", scriptContent)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to execute say command: %v\n%s", err, output)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Successfully spoke: %s", text)), nil
	}
	return tool, handler
}

// RegisterTools registers all tools with the server
func RegisterTools(s *server.MCPServer) {
	tool, handler := NewHelloTool()
	s.AddTool(tool, handler)

	colorTool, colorHandler := NewEnumTool()
	s.AddTool(colorTool, colorHandler)

	fetchTool, fetchHandler := NewFetchWebpageTool()
	s.AddTool(fetchTool, fetchHandler)

	ttsTool, ttsHandler := NewTextToSpeechTool()
	s.AddTool(ttsTool, ttsHandler)

	macSayTool, macSayHandler := NewMacSayTool()
	s.AddTool(macSayTool, macSayHandler)
}
