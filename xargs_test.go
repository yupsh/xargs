package xargs_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yupsh/xargs"
	"github.com/yupsh/xargs/opt"
)

// Mock command for testing
type mockCommand struct {
	args     []string
	executed int64
	delay    time.Duration
}

func (m *mockCommand) Execute(ctx context.Context, input any, output, stderr any) error {
	atomic.AddInt64(&m.executed, 1)

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Write args to output for verification
	if output != nil {
		if w, ok := output.(interface{ Write([]byte) (int, error) }); ok {
			w.Write([]byte(fmt.Sprintf("executed: %s\n", strings.Join(m.args, " "))))
		}
	}

	return nil
}

func mockFactory(params ...any) interface{} {
	args := make([]string, len(params))
	for i, p := range params {
		if s, ok := p.(string); ok {
			args[i] = s
		}
	}
	return &mockCommand{args: args}
}

func init() {
	// Register mock command for testing
	xargs.RegisterCommand("mock", func(params ...any) interface{} {
		return mockFactory(params...)
	})
}

// Example tests
func ExampleXargs() {
	ctx := context.Background()
	input := strings.NewReader("arg1\narg2\narg3")

	// Note: This would need a registered command to work
	cmd := xargs.Xargs("echo")
	cmd.Execute(ctx, input, os.Stdout, os.Stderr)
	// Output would depend on registered commands
}

// Basic functionality tests
func TestXargs_BasicExecution(t *testing.T) {
	ctx := context.Background()
	input := strings.NewReader("arg1 arg2\narg3 arg4")
	var output, stderr bytes.Buffer

	cmd := xargs.Xargs("mock")
	err := cmd.Execute(ctx, input, &output, &stderr)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should execute mock command with the arguments
	result := output.String()
	if !strings.Contains(result, "executed:") {
		t.Errorf("Expected execution output, got: %q", result)
	}
}

// Test MaxProcs parallel execution (CRITICAL NEW FUNCTIONALITY)
func TestXargs_ParallelExecution(t *testing.T) {
	tests := []struct {
		name     string
		maxProcs int
		numArgs  int
		delay    time.Duration
	}{
		{"sequential", 1, 4, 50 * time.Millisecond},
		{"parallel 2", 2, 4, 50 * time.Millisecond},
		{"parallel 4", 4, 8, 50 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register a command with delay for this test
			commandName := fmt.Sprintf("delay_mock_%s", tt.name)
			xargs.RegisterCommand(commandName, func(params ...any) interface{} {
				return &mockCommand{delay: tt.delay}
			})

			ctx := context.Background()

			// Create input with specified number of arguments
			args := make([]string, tt.numArgs)
			for i := 0; i < tt.numArgs; i++ {
				args[i] = fmt.Sprintf("arg%d", i)
			}
			input := strings.NewReader(strings.Join(args, "\n"))

			var output, stderr bytes.Buffer

			start := time.Now()
			cmd := xargs.Xargs(commandName, opt.MaxProcs(tt.maxProcs), opt.MaxArgs(1))
			err := cmd.Execute(ctx, input, &output, &stderr)
			duration := time.Since(start)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Calculate expected duration
			// Sequential: numArgs * delay
			// Parallel: ceil(numArgs/maxProcs) * delay
			expectedBatches := (tt.numArgs + tt.maxProcs - 1) / tt.maxProcs
			expectedDuration := time.Duration(expectedBatches) * tt.delay

			// Allow some tolerance for test timing
			tolerance := 100 * time.Millisecond
			if duration > expectedDuration+tolerance {
				t.Errorf("Execution took too long: expected ~%v, got %v (maxProcs=%d, numArgs=%d)",
					expectedDuration, duration, tt.maxProcs, tt.numArgs)
			}

			t.Logf("MaxProcs=%d, NumArgs=%d, Duration=%v, Expected=~%v",
				tt.maxProcs, tt.numArgs, duration, expectedDuration)
		})
	}
}

// Test context cancellation with parallel execution
func TestXargs_ParallelContextCancellation(t *testing.T) {
	// Register a slow command
	xargs.RegisterCommand("slow_mock", func(params ...any) interface{} {
		return &mockCommand{delay: 500 * time.Millisecond}
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Create multiple args that would take a long time sequentially
	input := strings.NewReader("arg1\narg2\narg3\narg4\narg5\narg6")
	var output, stderr bytes.Buffer

	// Use parallel execution
	cmd := xargs.Xargs("slow_mock", opt.MaxProcs(3), opt.MaxArgs(1))

	// Start execution in goroutine
	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute(ctx, input, &output, &stderr)
	}()

	// Cancel after short delay
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Verify cancellation
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Parallel execution did not respond to cancellation")
	}
}

// Test MaxArgs functionality
func TestXargs_MaxArgs(t *testing.T) {
	ctx := context.Background()
	input := strings.NewReader("a b c d e f")
	var output, stderr bytes.Buffer

	// Track execution count
	var execCount int64
	xargs.RegisterCommand("count_mock", func(params ...any) interface{} {
		atomic.AddInt64(&execCount, 1)
		return &mockCommand{args: make([]string, len(params))}
	})

	cmd := xargs.Xargs("count_mock", opt.MaxArgs(2))
	err := cmd.Execute(ctx, input, &output, &stderr)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should execute 3 times: (a,b), (c,d), (e,f)
	if execCount != 3 {
		t.Errorf("Expected 3 executions, got %d", execCount)
	}
}

// Test different input delimiters
func TestXargs_InputDelimiters(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		delimiter string
		nullDelim bool
		expected  []string
	}{
		{
			name:     "space delimited",
			input:    "a b c d",
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:      "custom delimiter",
			input:     "a,b,c,d",
			delimiter: ",",
			expected:  []string{"a", "b", "c", "d"},
		},
		{
			name:      "null delimited",
			input:     "a\x00b\x00c\x00d",
			nullDelim: true,
			expected:  []string{"a", "b", "c", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			input := strings.NewReader(tt.input)
			var output, stderr bytes.Buffer

			// Create command to capture arguments
			var capturedArgs []string
			xargs.RegisterCommand(fmt.Sprintf("capture_%s", tt.name), func(params ...any) interface{} {
				for _, p := range params {
					if s, ok := p.(string); ok {
						capturedArgs = append(capturedArgs, s)
					}
				}
				return &mockCommand{}
			})

			var cmd interface{}
			if tt.nullDelim {
				cmd = xargs.Xargs(fmt.Sprintf("capture_%s", tt.name), opt.NullDelim, opt.MaxArgs(1))
			} else if tt.delimiter != "" {
				cmd = xargs.Xargs(fmt.Sprintf("capture_%s", tt.name), opt.Delimiter(tt.delimiter), opt.MaxArgs(1))
			} else {
				cmd = xargs.Xargs(fmt.Sprintf("capture_%s", tt.name), opt.MaxArgs(1))
			}

			err := cmd.(interface {
				Execute(context.Context, any, any, any) error
			}).Execute(ctx, input, &output, &stderr)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(capturedArgs) != len(tt.expected) {
				t.Errorf("Expected %d args, got %d: %v", len(tt.expected), len(capturedArgs), capturedArgs)
			}

			for i, arg := range capturedArgs {
				if i < len(tt.expected) && arg != tt.expected[i] {
					t.Errorf("Arg %d: expected %q, got %q", i, tt.expected[i], arg)
				}
			}
		})
	}
}

// Test error handling
func TestXargs_ErrorHandling(t *testing.T) {
	t.Run("missing command", func(t *testing.T) {
		ctx := context.Background()
		input := strings.NewReader("test")
		var output, stderr bytes.Buffer

		cmd := xargs.Xargs() // No command specified
		err := cmd.Execute(ctx, input, &output, &stderr)

		if err == nil {
			t.Error("Expected error for missing command")
		}
	})

	t.Run("unregistered command", func(t *testing.T) {
		ctx := context.Background()
		input := strings.NewReader("test")
		var output, stderr bytes.Buffer

		cmd := xargs.Xargs("nonexistent_command")
		err := cmd.Execute(ctx, input, &output, &stderr)

		if err == nil {
			t.Error("Expected error for unregistered command")
		}
	})
}

// Benchmark parallel vs sequential execution
func BenchmarkXargs_Sequential(b *testing.B) {
	xargs.RegisterCommand("bench_mock", func(params ...any) interface{} {
		return &mockCommand{delay: 1 * time.Millisecond}
	})

	ctx := context.Background()
	input := strings.NewReader("a\nb\nc\nd\ne\nf\ng\nh")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output, stderr bytes.Buffer
		cmd := xargs.Xargs("bench_mock", opt.MaxProcs(1), opt.MaxArgs(1))
		cmd.Execute(ctx, input, &output, &stderr)
	}
}

func BenchmarkXargs_Parallel4(b *testing.B) {
	xargs.RegisterCommand("bench_mock_par", func(params ...any) interface{} {
		return &mockCommand{delay: 1 * time.Millisecond}
	})

	ctx := context.Background()
	input := strings.NewReader("a\nb\nc\nd\ne\nf\ng\nh")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output, stderr bytes.Buffer
		cmd := xargs.Xargs("bench_mock_par", opt.MaxProcs(4), opt.MaxArgs(1))
		cmd.Execute(ctx, input, &output, &stderr)
	}
}
