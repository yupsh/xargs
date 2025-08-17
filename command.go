package command

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	yup "github.com/gloo-foo/framework"
)

type command Inputs[flags]

func Xargs(parameters ...any) yup.Command {
	cmd := command(args[flags](parameters...))
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

func (p command) Executor() yup.CommandExecutor {
	return func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
		scanner := bufio.NewScanner(stdin)

		for scanner.Scan() {
			line := scanner.Text()

			if p.commands == nil && len(p.args) == 0 {
				fmt.Fprintln(stdout, line)
				continue
			}

			if line == "" && p.Flags.NoRunEmpty {
				continue
			}

			var inputArgs []string
			if p.Flags.Delimiter != "" {
				inputArgs = strings.Split(line, string(p.Flags.Delimiter))
			} else if p.Flags.NullDelim {
				inputArgs = strings.Split(line, "\x00")
			} else {
				inputArgs = strings.Fields(line)
			}

			var cmdArgs []string
			cmdArgs = append(cmdArgs, p.args...)

			if p.Flags.ReplaceStr != "" {
				replaceStr := string(p.Flags.ReplaceStr)
				for i, arg := range cmdArgs {
					cmdArgs[i] = strings.ReplaceAll(arg, replaceStr, strings.Join(inputArgs, " "))
				}
			} else {
				cmdArgs = append(cmdArgs, inputArgs...)
			}

			if bool(p.Flags.Verbose) || bool(p.Flags.Print) {
				cmdLine := strings.Join(cmdArgs, " ")
				if p.Flags.Print {
					fmt.Fprintln(stderr, cmdLine)
					continue
				}
				fmt.Fprintln(stderr, cmdLine)
			}

			var (
				wg        sync.WaitGroup
				mu        sync.Mutex
				outputs   []string
				cmdErrors    []string
				semaphore = make(chan struct{}, int(p.Flags.MaxProcs))
			)

			for _, cmd := range p.commands {
				wg.Add(1)
				semaphore <- struct{}{}

				go func(cmd yup.Command) {
					defer wg.Done()
					defer func() { <-semaphore }()

					// Create IO streams
					input := strings.NewReader(strings.Join(cmdArgs, " "))
					var outBuf, errBuf bytes.Buffer

					// Execute the command
					executor := cmd.Executor()
					execErr := executor(ctx, input, &outBuf, &errBuf)

					// Collect results with mutex protection
					mu.Lock()
					defer mu.Unlock()

					if outBuf.Len() > 0 {
						outputs = append(outputs, strings.TrimRight(outBuf.String(), "\n"))
					}
					if errBuf.Len() > 0 {
						cmdErrors = append(cmdErrors, strings.TrimRight(errBuf.String(), "\n"))
					}
					if execErr != nil {
						cmdErrors = append(cmdErrors, fmt.Sprintf("error: %v", execErr))
					}
				}(cmd)
			}

			wg.Wait()

			stdoutResult := strings.Join(outputs, "\n")
			stderrResult := strings.Join(cmdErrors, "\n")

			if stdoutResult != "" {
				fmt.Fprintln(stdout, stdoutResult)
			}
			if stderrResult != "" {
				fmt.Fprintln(stderr, stderrResult)
			}
		}

		return scanner.Err()
	}
}

type Inputs[O any] struct {
	Flags    O
	commands []yup.Command
	args     []string
}

func args[O any](parameters ...any) (result Inputs[O]) {
	var options []yup.Switch[O]

	for _, arg := range parameters {
		switch v := arg.(type) {
		case yup.Switch[O]:
			options = append(options, v)
		case yup.Command:
			result.commands = append(result.commands, v)
		case string:
			result.args = append(result.args, v)
		default:
			slog.Warn("Unknown argument type", "arg", v, "type", fmt.Sprintf("%T/%T", arg, v))
		}
	}

	result.Flags = configure(options...)
	return result
}

func configure[T any](opts ...yup.Switch[T]) T {
	def := new(T)
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.Configure(def)
	}
	return *def
}
