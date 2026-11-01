package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceHashIsTheOneTheGoldenLocksRecord(t *testing.T) {
	shim, err := os.ReadFile(filepath.Join("..", "..", "test", "golden", "sample-codex", "expected", ".ai", "bin", "guard"))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := SourceHash()
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) != 64 || Shim(hash) != string(shim) {
		t.Errorf("hash %s gives a shim other than the golden one", hash)
	}
}

func TestShimNamesTheHashFolderAndTheInstallCommand(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	shim := Shim(hash)

	for _, part := range []string{
		"#!/bin/sh\n",
		`guard="${DASHCREW_HOME:-$HOME/.dashcrew}/guard/0123456789abcdef/guard"`,
		"dashcrew guard install --expect " + hash + " </dev/null",
		"exit 0\n",
	} {
		if !strings.Contains(shim, part) {
			t.Errorf("shim misses %q", part)
		}
	}
}

func TestDefaultHome(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
		ok   bool
	}{
		{map[string]string{"DASHCREW_HOME": "/x", "HOME": "/h"}, "/x", true},
		{map[string]string{"DASHCREW_HOME": "", "HOME": "/h"}, filepath.Join("/h", ".dashcrew"), true},
		{map[string]string{"USERPROFILE": "/u"}, filepath.Join("/u", ".dashcrew"), true},
		{map[string]string{}, "", false},
	}
	for _, c := range cases {
		got, ok := DefaultHome(c.env)
		if got != c.want || ok != c.ok {
			t.Errorf("DefaultHome(%v) = %q, %v", c.env, got, ok)
		}
	}
	if got := BinaryPath("/home", "0123456789abcdef99"); got != filepath.Join("/home", "guard", "0123456789abcdef", "guard") {
		t.Errorf("BinaryPath %s", got)
	}
}
