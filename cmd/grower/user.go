package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/reilandeubank/grower/internal/auth"
)

func cmdUser(args []string, stdin io.Reader, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: user needs a subcommand (create, set-password)", errUsage)
	}
	sub := args[0]
	if sub != "create" && sub != "set-password" {
		return fmt.Errorf("%w: unknown user subcommand %q", errUsage, sub)
	}

	flags := flag.NewFlagSet("user "+sub, flag.ContinueOnError)
	flags.SetOutput(stderr)
	username := flags.String("username", "", "account name (required)")
	fromStdin := flags.Bool("password-stdin", false, "read the password as one line from stdin (for scripts)")
	if err := flags.Parse(args[1:]); err != nil {
		return fmt.Errorf("%w: %w", errUsage, err)
	}
	if *username == "" {
		return fmt.Errorf("%w: --username is required", errUsage)
	}
	if err := auth.ValidateUsername(*username); err != nil {
		return err
	}

	password, err := readPassword(stdin, stderr, *fromStdin)
	if err != nil {
		return err
	}
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx := context.Background()
	d, err := openMigrated(ctx, cfg)
	if err != nil {
		return err
	}
	defer d.Close()
	svc, err := auth.NewService(d, auth.Options{})
	if err != nil {
		return err
	}

	if sub == "create" {
		if _, err := svc.CreateUser(ctx, *username, password, cfg.Location.String()); err != nil {
			return fmt.Errorf("create user %q: %w", *username, err)
		}
		fmt.Fprintf(stderr, "created user %q (time zone %s)\n", *username, cfg.Location)
		return nil
	}
	if err := svc.SetPassword(ctx, *username, password); err != nil {
		return fmt.Errorf("set password for %q: %w", *username, err)
	}
	fmt.Fprintf(stderr, "password updated for %q; existing sessions were signed out\n", *username)
	return nil
}

// readPassword reads one line from stdin for scripts, or prompts twice without echo.
func readPassword(stdin io.Reader, stderr io.Writer, fromStdin bool) (string, error) {
	if fromStdin {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		// EOF after a final unterminated line is fine (e.g. printf without \n).
		if err != nil && (!errors.Is(err, io.EOF) || line == "") {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("stdin is not a terminal; pass --password-stdin to read the password from it")
	}
	fmt.Fprintf(stderr, "Password (at least %d characters): ", auth.MinPasswordLength)
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	fmt.Fprint(stderr, "Repeat password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if string(first) != string(second) {
		return "", errors.New("passwords do not match")
	}
	return string(first), nil
}
