package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const writePrompt = "This command changes SigNoz. Type 'write' to confirm: "

func (a *app) requireHumanWrite(cmd *cobra.Command, what string) error {
	writeFlag := false
	if cmd != nil {
		f := cmd.Flags().Lookup("write")
		if f != nil {
			writeFlag = f.Changed && f.Value.String() == "true"
		}
	}
	if !writeFlag {
		return authErr(fmt.Errorf("%s is read-only for agents; a human must run this command with --write in an interactive terminal", what))
	}
	if !isTerminal(a.in) {
		return authErr(fmt.Errorf("%s with --write requires an interactive terminal; agents cannot perform writes", what))
	}
	fmt.Fprint(a.errOut, writePrompt)
	var ans string
	if _, err := fmt.Fscanln(a.in, &ans); err != nil {
		return authErr(errors.New("write confirmation cancelled"))
	}
	if strings.TrimSpace(ans) != "write" {
		return authErr(errors.New("write confirmation denied"))
	}
	return nil
}

func bindWriteFlag(cmd *cobra.Command, write *bool) {
	cmd.Flags().BoolVar(write, "write", false, "allow this mutating operation (interactive terminal; type write when prompted)")
}

func isTerminalFile(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
