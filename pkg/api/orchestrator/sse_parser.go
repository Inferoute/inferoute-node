package orchestrator

import (
	"bufio"
	"encoding/json"
	"strings"
)

// SSEDataEvent represents the JSON structure typically found in an SSE data field for chat completions.
// We are only interested in the delta content for reconstructing the full message.
type SSEDataEvent struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// parseSSEStreamToText parses a string of Server-Sent Events (SSE)
// and reconstructs the complete output text from chat completion deltas.
// It specifically looks for `data:` lines and concatenates the `choices[0].delta.content`.
func parseSSEStreamToText(sseData string) (string, error) {
	var fullText strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(sseData))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "data:") {
			jsonData := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

			// Skip empty data lines or special [DONE] message
			if jsonData == "" || jsonData == "[DONE]" {
				continue
			}

			var event SSEDataEvent
			if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
				// Not all data lines might be JSON or match our expected structure, so we can log and continue
				// For token counting, we only care about the content deltas.
				// fmt.Printf("Error unmarshalling SSE data line: %v, data: %s\n", err, jsonData)
				continue
			}

			if len(event.Choices) > 0 && event.Choices[0].Delta.Content != "" {
				fullText.WriteString(event.Choices[0].Delta.Content)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return fullText.String(), nil
}
