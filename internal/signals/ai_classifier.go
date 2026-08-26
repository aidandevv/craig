package signals

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aidandevaney/apartment-hunter/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)

// jsonCodec lets the Go and Python sidecars use the documented gRPC service
// without checked-in generated artifacts. The canonical .proto remains the
// contract and can be code-generated later without changing this boundary.
type jsonCodec struct{}

func (jsonCodec) Name() string                       { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

type AIClassifier struct {
	address string
	timeout time.Duration
}

func NewAIClassifier(address string) *AIClassifier {
	encoding.RegisterCodec(jsonCodec{})
	return &AIClassifier{address: address, timeout: 2500 * time.Millisecond}
}
func (*AIClassifier) Name() string { return "ai_classifier" }
func (a *AIClassifier) Evaluate(ctx context.Context, listing domain.Listing) (domain.SignalResult, error) {
	if len(listing.Images) == 0 || a.address == "" {
		return domain.SignalResult{Name: a.Name()}, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	connection, err := grpc.DialContext(callCtx, a.address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})), grpc.WithBlock())
	if err != nil {
		return domain.SignalResult{Name: a.Name(), Details: []string{"sidecar_unavailable"}}, nil
	}
	defer connection.Close()
	request := map[string]string{"image_url": listing.Images[0]}
	response := struct {
		AIScore    float64 `json:"ai_score"`
		Confidence float64 `json:"confidence"`
		LatencyMS  int64   `json:"latency_ms"`
		Error      string  `json:"error"`
	}{}
	if err = connection.Invoke(callCtx, "/inference.ImageClassifier/ClassifyAIGenerated", request, &response); err != nil {
		return domain.SignalResult{}, fmt.Errorf("gRPC classify: %w", err)
	}
	result := domain.SignalResult{Name: a.Name(), Risk: response.AIScore * .40, Details: []string{fmt.Sprintf("ai_score=%.2f confidence=%.2f", response.AIScore, response.Confidence)}}
	if response.AIScore >= .80 {
		result.Flags = []string{"ai_generated_image"}
	}
	return result, nil
}
