// Package cli is the command line of the container (#23, ADR-0006): the emergency entry for an
// administrator who is locked out. It runs inside the container, next to the server, and
// talks to the same database; it is not an HTTP endpoint and has no login of its own.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/admax12959/X-Project/internal/domain"
)

// Commands is the list of subcommand names, so main knows when to run Run instead of the server.
var Commands = []string{"reset-user"}

// Usage is printed on a wrong command line.
const Usage = `usage:
  xproject reset-user --user <username|login email> [--second-factor] [--password]

  reset-user   removes the user's second factor (--second-factor) and/or gives the user a new
               random temporary password (--password), printed once on standard output. The
               password has to be changed at the next login and every session of the user
               ends. Both are written to the audit log with the command line as operator.`

// Run runs one command and returns the process exit status: 0 on success, 1 when the command
// failed, 2 for a wrong command line.
func Run(ctx context.Context, svc *domain.Service, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "reset-user" {
		fmt.Fprintln(stderr, Usage)
		return 2
	}
	return resetUser(ctx, svc, args[1:], stdout, stderr)
}

func resetUser(ctx context.Context, svc *domain.Service, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reset-user", flag.ContinueOnError)
	fs.SetOutput(stderr)
	user := fs.String("user", "", "username or login email of the user")
	secondFactor := fs.Bool("second-factor", false, "remove the user's second factor")
	password := fs.Bool("password", false, "give the user a new temporary password")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *user == "" || fs.NArg() > 0 || (!*secondFactor && !*password) {
		fmt.Fprintln(stderr, Usage)
		return 2
	}
	res, err := svc.ResetUserByCLI(ctx, domain.CLIResetInput{User: *user, SecondFactor: *secondFactor, Password: *password})
	if err != nil {
		if errors.Is(err, domain.ErrCLIUserNotFound) || errors.Is(err, domain.ErrCLINoPassword) || errors.Is(err, domain.ErrCLINothingAsked) {
			fmt.Fprintf(stderr, "reset-user: %v\n", err)
		} else {
			fmt.Fprintf(stderr, "reset-user: failed: %v\n", err)
		}
		return 1
	}
	if *secondFactor {
		if res.SecondFactorReset {
			fmt.Fprintf(stdout, "Second factor of %s removed.\n", res.User)
		} else {
			fmt.Fprintf(stdout, "%s has no second factor; nothing to reset.\n", res.User)
		}
	}
	if *password {
		fmt.Fprintf(stdout, "Temporary password for %s: %s\n", res.User, res.Password)
		fmt.Fprintf(stdout, "It must be changed at the next login; every session of %s has ended.\n", res.User)
	}
	return 0
}
