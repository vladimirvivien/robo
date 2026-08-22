package router

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
)

// Strategy represents the routing decision strategy.
type Strategy string

const (
	// StrategyAuto performs speculative local execution with cloud escalation.
	StrategyAuto Strategy = "auto"
	// StrategyLocalOnly restricts execution strictly to the local on-device engine.
	StrategyLocalOnly Strategy = "local-only"
	// StrategyCloudOnly restricts execution strictly to cloud frontier models.
	StrategyCloudOnly Strategy = "cloud-only"
)

// Router coordinates between local and cloud engines based on rules and heuristics.
type Router struct {
	local engine.Engine
	cloud engine.Engine
	cfg   config.LLMConfig
}

// NewRouter creates a new hybrid Router instance.
func NewRouter(local engine.Engine, cloud engine.Engine, cfg config.LLMConfig) *Router {
	if cfg.MaxLocalTokens <= 0 {
		cfg.MaxLocalTokens = 4096
	}

	if !cfg.Cloud.Enabled {
		cloud = nil
	}

	return &Router{
		local: local,
		cloud: cloud,
		cfg:   cfg,
	}
}

// Name returns the router engine identifier.
func (r *Router) Name() string {
	return "router"
}

// DecideRoute determines the target execution strategy for a given request.
func (r *Router) DecideRoute(req engine.Request) (Strategy, string) {
	// 1. Explicit request flag overrides
	switch strings.ToLower(req.ForceBackend) {
	case "local", "local-only", "--local", "--local-only":
		return StrategyLocalOnly, "explicit flag: --local-only"
	case "cloud", "cloud-only", "--cloud", "--cloud-only":
		return StrategyCloudOnly, "explicit flag: --cloud-only"
	}

	// 2. Disabled model checks
	if !r.cfg.Local.Enabled && r.cfg.Cloud.Enabled {
		return StrategyCloudOnly, "configuration: local model is disabled"
	}
	if !r.cfg.Cloud.Enabled {
		return StrategyLocalOnly, "configuration: cloud model is disabled"
	}
	if !r.cfg.Local.Enabled && !r.cfg.Cloud.Enabled {
		return StrategyLocalOnly, "configuration: local default"
	}

	// 3. Multimodal image attachments require cloud vision models
	if len(req.Images) > 0 {
		return StrategyCloudOnly, "heuristic: multimodal image attachments require cloud vision"
	}

	// 4. Context size exceeding local token ceiling
	estTokens := EstimateTokens(req)
	if r.cfg.MaxLocalTokens > 0 && estTokens > r.cfg.MaxLocalTokens {
		return StrategyCloudOnly, fmt.Sprintf("heuristic: context size (%d tokens) exceeds local limit (%d)", estTokens, r.cfg.MaxLocalTokens)
	}

	// 5. Default auto-routing (both enabled or auto_route: true)
	return StrategyAuto, "strategy: automatic intelligent routing"
}

// EscalateToCloudSignal is the control tag output by the local SLM when a task exceeds local capacity.
const EscalateToCloudSignal = "[ESCALATE_TO_CLOUD]"

// Generate dispatches unary generation to local or cloud engine with escalation.
func (r *Router) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	strategy, _ := r.DecideRoute(req)

	switch strategy {
	case StrategyLocalOnly:
		if r.local == nil {
			return nil, errors.New("router: local engine is not available")
		}
		return r.local.Generate(ctx, req)

	case StrategyCloudOnly:
		if r.cloud == nil || !r.cfg.Cloud.Enabled {
			return nil, errors.New("router: cloud engine is not available or disabled")
		}
		return r.cloud.Generate(ctx, req)

	case StrategyAuto:
		fallthrough
	default:
		if r.local != nil {
			resp, err := r.local.Generate(ctx, req)
			if err == nil {
				// If local model actively signals that task exceeds local capacity, delegate to cloud if enabled
				if strings.HasPrefix(strings.TrimSpace(resp.Text), EscalateToCloudSignal) && r.cloud != nil && r.cfg.Cloud.Enabled {
					return r.cloud.Generate(ctx, req)
				}
				return resp, nil
			}
			if r.cloud == nil || !r.cfg.Cloud.Enabled {
				return nil, err
			}
			// Attempt cloud fallback on error if cloud is enabled
			cloudResp, cloudErr := r.cloud.Generate(ctx, req)
			if cloudErr == nil {
				return cloudResp, nil
			}
			return nil, fmt.Errorf("local execution failed (%w); cloud fallback also failed: %v", err, cloudErr)
		}
		if r.cloud == nil || !r.cfg.Cloud.Enabled {
			return nil, errors.New("router: no active engine available")
		}
		return r.cloud.Generate(ctx, req)
	}
}

// GenerateStream dispatches streaming generation with seamless escalation.
func (r *Router) GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
	strategy, _ := r.DecideRoute(req)

	switch strategy {
	case StrategyLocalOnly:
		if r.local == nil {
			return nil, errors.New("router: local engine is not available")
		}
		stream, err := r.local.GenerateStream(ctx, req)
		if err != nil {
			return nil, err
		}
		return r.forwardWithEscalation(ctx, stream, nil, req)

	case StrategyCloudOnly:
		if r.cloud == nil || !r.cfg.Cloud.Enabled {
			return nil, errors.New("router: cloud engine is not available or disabled")
		}
		return r.cloud.GenerateStream(ctx, req)

	case StrategyAuto:
		fallthrough
	default:
		var fallbackEngine engine.Engine
		if r.cfg.Cloud.Enabled {
			fallbackEngine = r.cloud
		}

		if r.local != nil {
			stream, err := r.local.GenerateStream(ctx, req)
			if err == nil {
				return r.forwardWithEscalation(ctx, stream, fallbackEngine, req)
			}
			if fallbackEngine == nil {
				return nil, err
			}
			cloudStream, cloudErr := fallbackEngine.GenerateStream(ctx, req)
			if cloudErr == nil {
				return cloudStream, nil
			}
			return nil, fmt.Errorf("local engine failed (%w); cloud fallback also failed: %v", err, cloudErr)
		}
		if fallbackEngine == nil {
			return nil, errors.New("router: no active engine available")
		}
		return fallbackEngine.GenerateStream(ctx, req)
	}
}

// forwardWithEscalation monitors the primary stream and escalates to fallback engine on error or [ESCALATE_TO_CLOUD] signal.
func (r *Router) forwardWithEscalation(ctx context.Context, primary <-chan engine.StreamChunk, fallback engine.Engine, req engine.Request) (<-chan engine.StreamChunk, error) {
	out := make(chan engine.StreamChunk, 16)

	go func() {
		defer close(out)

		var buffer []engine.StreamChunk
		var accumulated strings.Builder
		buffered := true

		for chunk := range primary {
			if chunk.Error != nil {
				// Immediate failure on stream: if fallback available, escalate
				if fallback != nil && accumulated.Len() == 0 {
					fbStream, err := fallback.GenerateStream(ctx, req)
					if err == nil {
						for fbChunk := range fbStream {
							out <- fbChunk
						}
						return
					}
				}
				out <- chunk
				return
			}

			if buffered {
				buffer = append(buffer, chunk)
				accumulated.WriteString(chunk.Text)

				trimmed := strings.TrimSpace(accumulated.String())
				if strings.HasPrefix(trimmed, EscalateToCloudSignal) {
					// Active signal detected: discard local buffer and escalate to cloud
					if fallback != nil {
						fbStream, err := fallback.GenerateStream(ctx, req)
						if err == nil {
							for fbChunk := range fbStream {
								out <- fbChunk
							}
							return
						}
					}
				}

				// If accumulated buffer exceeds the signal length and is not a prefix, flush and stop buffering
				if accumulated.Len() >= len(EscalateToCloudSignal) {
					buffered = false
					for _, bChunk := range buffer {
						out <- bChunk
					}
					buffer = nil
				}
				continue
			}

			out <- chunk
		}

		// Flush any remaining buffered chunks if stream closed before threshold
		if buffered && len(buffer) > 0 {
			for _, bChunk := range buffer {
				out <- bChunk
			}
		}
	}()

	return out, nil
}

// Close closes both underlying engines if present.
func (r *Router) Close() error {
	var errs []error
	if r.local != nil {
		if err := r.local.Close(); err != nil {
			errs = append(errs, fmt.Errorf("local engine close: %w", err))
		}
	}
	if r.cloud != nil {
		if err := r.cloud.Close(); err != nil {
			errs = append(errs, fmt.Errorf("cloud engine close: %w", err))
		}
	}
	return errors.Join(errs...)
}

// EstimateTokens provides a fast, heuristic approximation of token count.
func EstimateTokens(req engine.Request) int {
	chars := len(req.Prompt) + len(req.SystemPrompt)
	for _, f := range req.ContextFiles {
		chars += len(f.Path) + len(f.Content)
	}
	return (chars + 3) / 4
}
