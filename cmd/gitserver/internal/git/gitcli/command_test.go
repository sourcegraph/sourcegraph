package gitcli

import (
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"
)

func TestArgsFromArguments(t *testing.T) {
	g := &gitCLIBackend{logger: logtest.Scoped(t)}

	t.Run("SafeFlagArgument", func(t *testing.T) {
		args, err := g.argsFromArguments("subcommand", []Argument{FlagArgument{string: "--flag"}})
		require.NoError(t, err)
		require.Equal(t, []string{"subcommand", "--flag"}, args)
	})

	t.Run("SpecSafeValueArgument", func(t *testing.T) {
		args, err := g.argsFromArguments("rev-parse", []Argument{SpecSafeValueArgument{string: "HEAD"}})
		require.NoError(t, err)
		require.Equal(t, []string{"rev-parse", "HEAD"}, args)

		_, err = g.argsFromArguments("rev-parse", []Argument{SpecSafeValueArgument{string: "--not-allowed"}})
		require.Error(t, err)
	})

	t.Run("ConfigArgument", func(t *testing.T) {
		args, err := g.argsFromArguments("subcommand", []Argument{ConfigArgument{Key: "key", Value: "value"}})
		require.NoError(t, err)
		require.Equal(t, []string{"-c", "key=value", "subcommand"}, args)
	})

	t.Run("SafeValueFlagArgument", func(t *testing.T) {
		args, err := g.argsFromArguments("subcommand", []Argument{ValueFlagArgument{Flag: "--flag", Value: "value"}})
		require.NoError(t, err)
		require.Equal(t, []string{"subcommand", "--flag=value"}, args)
	})

	t.Run("MultipleArguments", func(t *testing.T) {
		args, err := g.argsFromArguments("subcommand", []Argument{
			FlagArgument{string: "--flag1"},
			SpecSafeValueArgument{string: "HEAD"},
			ValueFlagArgument{Flag: "--flag2", Value: "value"},
			ConfigArgument{Key: "key", Value: "value"},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"-c", "key=value", "subcommand", "--flag1", "HEAD", "--flag2=value"}, args)
	})
}
