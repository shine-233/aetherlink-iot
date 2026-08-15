// Command hashgen generates a bcrypt hash for a password supplied as an
// argument. It is intended for local account bootstrap and never stores the
// plaintext password in source or configuration files.
package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] == "" {
		fmt.Fprintln(stderr, "usage: go run ./cmd/hashgen <password>")
		return 2
	}

	password := []byte(args[0])
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := bcrypt.CompareHashAndPassword(hash, password); err != nil {
		fmt.Fprintln(stderr, "self-check failed")
		return 1
	}

	fmt.Fprintln(stdout, string(hash))
	return 0
}
