package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MockEngine is a test double for the Engine interface.
type MockEngine struct {
	EngineName        string
	GenerateFn        func(ctx context.Context, req Request) (*Response, error)
	GenerateStreamFn  func(ctx context.Context, req Request) (<-chan StreamChunk, error)
	Calls             []Request
	Closed            bool
	Mu                sync.Mutex
	ArtificialLatency time.Duration
}

// NewMockEngine creates a MockEngine initialized with standard echo behavior.
func NewMockEngine(name string) *MockEngine {
	m := &MockEngine{
		EngineName: name,
	}
	m.GenerateFn = func(ctx context.Context, req Request) (*Response, error) {
		if m.ArtificialLatency > 0 {
			select {
			case <-time.After(m.ArtificialLatency):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return &Response{
			Text:       fmt.Sprintf("[%s] %s", name, req.Prompt),
			Provider:   name,
			Model:      "mock-model",
			UsedLocal:  strings.Contains(name, "local") || strings.Contains(name, "litertlm"),
			TokensUsed: len(req.Prompt) / 4,
		}, nil
	}
	m.GenerateStreamFn = func(ctx context.Context, req Request) (<-chan StreamChunk, error) {
		ch := make(chan StreamChunk, 10)
		go func() {
			defer close(ch)
			words := strings.Fields(req.Prompt)
			for _, w := range words {
				if m.ArtificialLatency > 0 {
					select {
					case <-time.After(m.ArtificialLatency / time.Duration(len(words)+1)):
					case <-ctx.Done():
						ch <- StreamChunk{Error: ctx.Err()}
						return
					}
				}
				ch <- StreamChunk{Text: w + " "}
			}
			ch <- StreamChunk{Final: true, TokensUsed: len(words)}
		}()
		return ch, nil
	}
	return m
}

func (m *MockEngine) Name() string {
	if m.EngineName == "" {
		return "mock"
	}
	return m.EngineName
}

func (m *MockEngine) Generate(ctx context.Context, req Request) (*Response, error) {
	m.Mu.Lock()
	m.Calls = append(m.Calls, req)
	m.Mu.Unlock()

	if m.GenerateFn != nil {
		return m.GenerateFn(ctx, req)
	}
	return nil, fmt.Errorf("mock: GenerateFn not set")
}

func (m *MockEngine) GenerateStream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	m.Mu.Lock()
	m.Calls = append(m.Calls, req)
	m.Mu.Unlock()

	if m.GenerateStreamFn != nil {
		return m.GenerateStreamFn(ctx, req)
	}
	return nil, fmt.Errorf("mock: GenerateStreamFn not set")
}

func (m *MockEngine) Close() error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.Closed = true
	return nil
}
