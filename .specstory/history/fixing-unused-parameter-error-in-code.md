## SpecStory

## Fixing Unused Parameter Error in Code (31/01/2025, 12:17:12)

_**User**_

For the code present, we get this error:
```
unused parameter: ctx
```
How can I resolve this? If you propose a fix, please make it concise.

---

_**Assistant**_

The error occurs because the `ctx` parameter in `generateHMAC` isn't being used. Since we're loading config directly, we can remove the unused parameter:

```go:pkg/api/orchestrator/service.go
// ... existing code ...
func (s *Service) generateHMAC(_ context.Context, consumerID uuid.UUID, req *OpenAIRequest) (string, error) {
	// Get HMAC secret from config
	cfg, err := config.LoadConfig("")
// ... existing code ...
```

Or if you prefer to remove it entirely:

```go:pkg/api/orchestrator/service.go
// ... existing code ...
func (s *Service) generateHMAC(consumerID uuid.UUID, req *OpenAIRequest) (string, error) {
	// Get HMAC secret from config
	cfg, err := config.LoadConfig("")
// ... existing code ...
```

Note: You'll need to update the caller if you choose the second option.

---



---

