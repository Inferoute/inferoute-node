package orchestrator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sentnl/inferoute-node/pkg/common"
)

// SSEDataEvent represents the JSON structure typically found in an SSE data field for chat completions.
// We are only interested in the delta content for reconstructing the full message.
type SSEDataEvent struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		ID *string `json:"id,omitempty"`
	} `json:"choices"`
	ID     string `json:"id,omitempty"`
	Object string `json:"object,omitempty"`
	Model  string `json:"model,omitempty"`
}

// parseSSEStreamToText parses a string of Server-Sent Events (SSE)
// and reconstructs the complete output text from chat completion deltas.
// It specifically looks for `data:` lines and concatenates the `choices[0].delta.content`.
func parseSSEStreamToText(sseData string, logger *common.Logger) (string, error) {
	var fullText strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(sseData))
	lineNum := 0

	logger.Info("[SSE Parser] Starting to parse SSE data. Total length: %d", len(sseData))
	if len(sseData) < 2000 {
		logger.Info("[SSE Parser] Captured SSE Data:\n%s", sseData)
	} else {
		logger.Info("[SSE Parser] Captured SSE Data is too long to log fully. First 2000 chars:\n%s", sseData[:2000])
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if strings.HasPrefix(line, "data:") {
			jsonData := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

			if jsonData == "" || jsonData == "[DONE]" {
				logger.Info("[SSE Parser] Skipping empty or [DONE] data line.")
				continue
			}

			var event SSEDataEvent
			if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
				logger.Error("[SSE Parser] Error unmarshalling SSE data line: %v, data: %s", err, jsonData)
				continue
			}

			if len(event.Choices) > 0 {
				if event.Choices[0].Delta.Content != "" {
					fullText.WriteString(event.Choices[0].Delta.Content)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Error("[SSE Parser] Scanner error: %v", err)
		return "", fmt.Errorf("scanner error while parsing SSE: %w", err)
	}
	finalParsedText := fullText.String()
	logger.Info("[SSE Parser] Finished parsing. Final reconstructed text length: %d", len(finalParsedText))
	if len(finalParsedText) > 0 && len(finalParsedText) < 500 {
		logger.Info("[SSE Parser] Reconstructed text: %s", finalParsedText)
	}

	return finalParsedText, nil
}
