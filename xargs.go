package xargs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	yup "github.com/yupsh/framework"
	"github.com/yupsh/framework/opt"

	localopt "github.com/yupsh/xargs/opt"
)

// Flags represents the configuration options for the xargs command
type Flags = localopt.Flags

// Command implementation
type command opt.Inputs[string, Flags]

// CommandFactory is a function that creates a yupsh command
type CommandFactory func(parameters ...any) yup.Command

// CommandRegistry maps command names to their factory functions
var CommandRegistry = make(map[string]CommandFactory)

// RegisterCommand registers a command factory with the xargs system
func RegisterCommand(name string, factory CommandFactory) {
	CommandRegistry[name] = factory
}

// Xargs creates a new xargs command with the given parameters
// First parameter is the command name, rest are fixed arguments
func Xargs(parameters ...any) yup.Command {
	cmd := command(opt.Args[string, Flags](parameters...))
	// Set defaults
	if cmd.Flags.MaxArgs == 0 {
		cmd.Flags.MaxArgs = 256
	}
	if cmd.Flags.MaxChars == 0 {
		cmd.Flags.MaxChars = 4096
	}
	if cmd.Flags.MaxProcs == 0 {
		cmd.Flags.MaxProcs = 1
	}
	return cmd
}

func (c command) Execute(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	// Check for cancellation before starting
	if err := yup.CheckContextCancellation(ctx); err != nil {
		return err
	}

	if len(c.Positional) == 0 {
		fmt.Fprintln(stderr, "xargs: missing command")
		return fmt.Errorf("missing command")
	}

	cmdName := c.Positional[0]
	baseArgs := c.Positional[1:]

	// Check if command is registered
	factory, exists := CommandRegistry[cmdName]
	if !exists {
		fmt.Fprintf(stderr, "xargs: command '%s' not found in registry\n", cmdName)
		return fmt.Errorf("command '%s' not found", cmdName)
	}

	// Read input and split into arguments
	args, err := c.readArgs(ctx, stdin)
	if err != nil {
		return err
	}

	if len(args) == 0 && bool(c.Flags.NoRunEmpty) {
		return nil
	}

	// Group arguments into command lines
	cmdLines := c.groupArgs(args, baseArgs)

	// Execute command lines with parallel processing
	return c.executeParallel(ctx, factory, cmdLines, stdout, stderr)
}

func (c command) readArgs(ctx context.Context, input io.Reader) ([]string, error) {
	var args []string

	if bool(c.Flags.NullDelim) {
		// Read null-delimited input
		data, err := c.readAllWithContext(ctx, input)
		if err != nil {
			return nil, err
		}
		parts := strings.Split(string(data), "\x00")
		for _, part := range parts {
			if part != "" {
				args = append(args, part)
			}
		}
	} else if c.Flags.Delimiter != "" {
		// Read with custom delimiter
		data, err := c.readAllWithContext(ctx, input)
		if err != nil {
			return nil, err
		}
		parts := strings.Split(string(data), string(c.Flags.Delimiter))
		for _, part := range parts {
			if part != "" {
				args = append(args, strings.TrimSpace(part))
			}
		}
	} else {
		// Read whitespace-delimited input (default)
		scanner := bufio.NewScanner(input)
		for yup.ScanWithContext(ctx, scanner) {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				fields := strings.Fields(line)
				args = append(args, fields...)
			}
		}

		// Check if context was cancelled
		if err := yup.CheckContextCancellation(ctx); err != nil {
			return args, err
		}

		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}

	return args, nil
}

func (c command) groupArgs(inputArgs, baseArgs []string) [][]string {
	var cmdLines [][]string

	for i := 0; i < len(inputArgs); {
		cmdArgs := make([]string, len(baseArgs))
		copy(cmdArgs, baseArgs)

		// Add as many input arguments as possible
		argsAdded := 0
		charsUsed := 0

		for j := i; j < len(inputArgs); j++ {
			arg := inputArgs[j]

			// Check limits
			if int(c.Flags.MaxArgs) > 0 && argsAdded >= int(c.Flags.MaxArgs) {
				break
			}
			if int(c.Flags.MaxChars) > 0 && charsUsed+len(arg)+1 > int(c.Flags.MaxChars) {
				break
			}

			cmdArgs = append(cmdArgs, arg)
			argsAdded++
			charsUsed += len(arg) + 1 // +1 for space
		}

		if len(cmdArgs) > len(baseArgs) || !bool(c.Flags.NoRunEmpty) {
			cmdLines = append(cmdLines, cmdArgs)
		}

		i += argsAdded
		if argsAdded == 0 {
			break // Avoid infinite loop
		}
	}

	return cmdLines
}

func (c command) executeCommand(ctx context.Context, factory CommandFactory, args []string, output, stderr io.Writer) error {
	if bool(c.Flags.Print) {
		fmt.Fprintf(stderr, "xargs: executing with args: %v\n", args)
	}

	if bool(c.Flags.Interactive) {
		fmt.Fprintf(stderr, "Execute command with args %v? [y/N] ", args)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			return nil
		}
	}

	// Convert strings to any for the factory function
	params := make([]any, len(args))
	for i, arg := range args {
		params[i] = arg
	}

	// Create the command using the factory
	cmd := factory(params...)

	// Execute the command with empty input (xargs provides the args, not stdin data)
	return cmd.Execute(ctx, strings.NewReader(""), output, stderr)
}

// executeParallel executes command lines in parallel using worker goroutines
func (c command) executeParallel(ctx context.Context, factory CommandFactory, cmdLines [][]string, output, stderr io.Writer) error {
	if len(cmdLines) == 0 {
		return nil
	}

	maxProcs := int(c.Flags.MaxProcs)
	if maxProcs <= 0 {
		maxProcs = 1
	}

	// If only one proc or one command, execute sequentially
	if maxProcs == 1 || len(cmdLines) == 1 {
		for _, cmdArgs := range cmdLines {
			if err := yup.CheckContextCancellation(ctx); err != nil {
				return err
			}
			if err := c.executeCommand(ctx, factory, cmdArgs, output, stderr); err != nil {
				return err
			}
		}
		return nil
	}

	// Create channels for work distribution
	jobChan := make(chan []string, len(cmdLines))
	errorChan := make(chan error, len(cmdLines))
	var wg sync.WaitGroup

	// Create context for coordinating worker cancellation
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start worker goroutines
	for i := 0; i < maxProcs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-workerCtx.Done():
					return
				case cmdArgs, ok := <-jobChan:
					if !ok {
						return
					}

					// Execute the command
					if err := c.executeCommand(workerCtx, factory, cmdArgs, output, stderr); err != nil {
						select {
						case errorChan <- err:
						case <-workerCtx.Done():
						}
						return
					}
				}
			}
		}()
	}

	// Send jobs to workers
	go func() {
		defer close(jobChan)
		for _, cmdArgs := range cmdLines {
			select {
			case <-workerCtx.Done():
				return
			case jobChan <- cmdArgs:
			}
		}
	}()

	// Wait for completion or error
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// Check for errors
	for err := range errorChan {
		cancel() // Cancel remaining work
		return err
	}

	// Final context check
	return yup.CheckContextCancellation(ctx)
}

// readAllWithContext reads all content from a reader with context cancellation support
func (c command) readAllWithContext(ctx context.Context, reader io.Reader) ([]byte, error) {
	var result []byte
	buffer := make([]byte, 32*1024) // 32KB buffer
	totalRead := 0

	for {
		// Check for cancellation before each read
		if err := yup.CheckContextCancellation(ctx); err != nil {
			return result, err
		}

		n, err := reader.Read(buffer)
		if n > 0 {
			result = append(result, buffer[:n]...)
			totalRead += n

			// Check for cancellation every 1MB read to avoid excessive overhead
			if totalRead%(1024*1024) == 0 {
				if err := yup.CheckContextCancellation(ctx); err != nil {
					return result, err
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

func (c command) String() string {
	return fmt.Sprintf("xargs %v", c.Positional)
}
