package utils

import (
	"context"
	"errors"
	"fmt"

	"github.com/alcionai/corso/pkg/repository"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// RequireProps validates the existence of the properties
//  in the map.  Expects the format map[propName]propVal.
func RequireProps(props map[string]string) error {
	for name, val := range props {
		if len(val) == 0 {
			return errors.New(name + " is required to perform this command")
		}
	}
	return nil
}

// CloseRepo handles closing a repo.
func CloseRepo(ctx context.Context, r *repository.Repository) {
	if err := r.Close(ctx); err != nil {
		fmt.Print("Error closing repository:", err)
	}
}

// HasNoFlagsAndShownHelp shows the Help output if no flags
// were provided to the command.  Returns true if the help
// was shown.
// Use for when the non-flagged usage of a command
// (ex: corso backup restore exchange) is expected to no-op.
func HasNoFlagsAndShownHelp(cmd *cobra.Command) bool {
	if cmd.Flags().NFlag() == 0 {
		cobra.CheckErr(cmd.Help())
		return true
	}
	return false
}

// AddCommand adds the subCommand to the parent, and returns
// both the subCommand and its pflags.
func AddCommand(parent, c *cobra.Command) (*cobra.Command, *pflag.FlagSet) {
	parent.AddCommand(c)
	return c, c.Flags()
}
