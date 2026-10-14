// Command grower is the plant care server and its admin tools.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	_ "time/tzdata" // embeds the zone database, so GROWER_TZ and user zones work on any base image
)

const usage = `usage: grower <command> [flags]

commands:
  serve                     run the web server (needs GROWER_BASE_URL)
  migrate                   apply database migrations and print their status
  user create --username NAME [--password-stdin]
  user set-password --username NAME [--password-stdin]
                            set a password and sign that user out everywhere
  healthcheck               exit 0 if the local server's /healthz is OK (Docker HEALTHCHECK)
  version                   print the build version

Configuration comes from GROWER_* environment variables; see docs.
`

// errUsage marks errors caused by bad arguments (exit status 2).
var errUsage = errors.New("usage error")

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "serve":
		err = cmdServe(args[1:])
	case "migrate":
		err = cmdMigrate(args[1:], stdout)
	case "user":
		err = cmdUser(args[1:], stdin, stderr)
	case "healthcheck":
		err = cmdHealthcheck(args[1:])
	case "version":
		err = cmdVersion(stdout)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		err = fmt.Errorf("%w: unknown command %q", errUsage, args[0])
	}
	if err == nil {
		return 0
	}
	fmt.Fprintf(stderr, "grower: %v\n", err)
	if errors.Is(err, errUsage) {
		fmt.Fprint(stderr, "\n"+usage)
		return 2
	}
	return 1
}
