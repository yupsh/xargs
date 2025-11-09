package command_test

import (
	"errors"
	"testing"

	"github.com/gloo-foo/testable/assertion"
	"github.com/gloo-foo/testable/run"
	command "github.com/yupsh/xargs"
)

func TestXargs_Basic(t *testing.T) {
	result := run.Command(command.Xargs()).
		WithStdinLines("a", "b", "c").Run()
	assertion.NoError(t, result.Err)
	assertion.Count(t, result.Stdout, 3)
}

func TestXargs_MaxArgs(t *testing.T) {
	result := run.Command(command.Xargs(command.MaxArgs(2))).
		WithStdinLines("a", "b", "c").Run()
	assertion.NoError(t, result.Err)
	assertion.Count(t, result.Stdout, 3)
}

func TestXargs_Null(t *testing.T) {
	result := run.Command(command.Xargs(command.NullDelim)).
		WithStdin("a\x00b\x00c").Run()
	assertion.NoError(t, result.Err)
	assertion.Count(t, result.Stdout, 3)
}

func TestXargs_EmptyInput(t *testing.T) {
	result := run.Quick(command.Xargs())
	assertion.NoError(t, result.Err)
	assertion.Empty(t, result.Stdout)
}

func TestXargs_InputError(t *testing.T) {
	result := run.Command(command.Xargs()).
		WithStdinError(errors.New("read failed")).Run()
	assertion.ErrorContains(t, result.Err, "read failed")
}

