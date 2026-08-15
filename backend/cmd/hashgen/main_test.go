package main

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing password"},
		{name: "empty password", args: []string{""}},
		{name: "multiple arguments", args: []string{"first", "second"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			if code := run(tt.args, &stdout, &stderr); code != 2 {
				t.Fatalf("run() exit code = %d, want 2", code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("run() stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "usage:") {
				t.Fatalf("run() stderr = %q, want usage", stderr.String())
			}
		})
	}
}

func TestRunGeneratesVerifiableHash(t *testing.T) {
	const password = "LocalBootstrapPassword1!"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := run([]string{password}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("run() stderr = %q, want empty", stderr.String())
	}

	hash := strings.TrimSpace(stdout.String())
	if hash == "" {
		t.Fatal("run() produced an empty hash")
	}
	if strings.Contains(hash, password) {
		t.Fatal("run() output contains the plaintext password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Fatalf("run() produced an unverifiable bcrypt hash: %v", err)
	}
}
