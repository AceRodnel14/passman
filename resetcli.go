package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// runResetPassword implements:
//
//	reset-password <master key>
//
// It asks for the username and the new password, then re-wraps that user's
// vault key under the new password. Their stored passwords are kept and all
// their sessions are signed out.
func runResetPassword(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: reset-password <master key>")
		return 2
	}
	masterKey := strings.TrimSpace(args[0])
	cfg := loadConfig()
	db := connect(cfg)
	defer db.Close()
	ctx := context.Background()

	if _, err := unlockEscrow(ctx, db, masterKey); errors.Is(err, ErrBadMasterKey) {
		fmt.Fprintln(os.Stderr, "error: that master key is not correct")
		return 1
	} else if errors.Is(err, ErrNotFound) {
		fmt.Fprintln(os.Stderr, "error: passman has not been set up yet")
		return 1
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	in := bufio.NewReader(os.Stdin)
	username := prompt(in, "Username: ", false)
	if _, err := userByName(ctx, db, username); errors.Is(err, ErrNotFound) {
		fmt.Fprintf(os.Stderr, "error: no user named %q\n", username)
		return 1
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	password := prompt(in, "New password: ", true)
	confirm := prompt(in, "Confirm new password: ", true)
	if err := validateNewPassword(password, confirm); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	if err := resetPasswordWithMasterKey(ctx, db, masterKey, username, password, nil); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("Password for %q has been reset. Their saved passwords are intact; existing sessions were signed out.\n", username)
	return 0
}

func prompt(in *bufio.Reader, label string, secret bool) string {
	fmt.Print(label)
	if secret && term.IsTerminal(int(os.Stdin.Fd())) {
		b, _ := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		return string(b)
	}
	line, _ := in.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}
