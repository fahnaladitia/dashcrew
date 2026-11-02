package spec

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ErrExists is returned by New for a spec folder that already exists.
var ErrExists = errors.New("spec folder exists")

const prdTemplate = `---
title: "{{title}}"
feature: {{feature}}
release: "{{release}}"
status: draft
rev: 1
repos: []
owners: []
---

# {{title}}: PRD

Owners and open question owners are roles (product, backend, mobile), never names or emails.

## Problem

Who has the problem and what it costs them today.

## Users

## Goals

## Scope

## Not doing

## Acceptance criteria

Each criterion starts its list item with an ID, numbered once and never renumbered. A dropped one stays, struck
through with its reason: ` + "`~~AC-3~~ dropped: <why>`" + `.

- AC-1 Given <state>, when <action>, then <observable result>.
`

const trdTemplate = `---
title: "{{title}}"
feature: {{feature}}
release: "{{release}}"
status: draft
rev: 1
repos: []
owners: []
---

# {{title}}: TRD

## Impact

Per repo and platform: what changes, with the platform's sections.

## Contracts

- C-1 <endpoint, event or message>: shape, errors, versioning.

## Data and migrations

## Rollout order

## Risks

## Decisions

- D-1 <decision>: the options weighed and the reason.

## Open questions

Each open question is a list item that ends with ` + "`owner: <role>`" + `; an approved spec has none without one.

- none
`

// Templates are the spec templates of a knowledge repo, slash-separated paths in the order they are written.
var Templates = []struct{ Path, Content string }{
	{"templates/prd.md", prdTemplate},
	{"templates/trd.md", trdTemplate},
}

var (
	featureName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	releaseName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// Unreleased is the release folder of a spec when the branch records no release.
const Unreleased = "unreleased"

// New writes specs/<release>/<feature>/prd.md and trd.md into clone from its templates, with status draft and rev 1;
// a missing template is written first from the built-in one. It returns the files written, slash-separated and
// relative to clone. It never runs git.
func New(clone, feature, release string) ([]string, error) {
	if !featureName.MatchString(feature) {
		return nil, fmt.Errorf("feature %q: use lowercase words joined by -, such as notes-sync", feature)
	}
	if !releaseName.MatchString(release) || strings.Contains(release, "..") {
		return nil, fmt.Errorf("release %q: use a version such as 1.4.0, or %s", release, Unreleased)
	}
	if info, err := os.Stat(clone); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("No knowledge clone at %s; run dashcrew knowledge pull.", clone)
	}
	folder := Dir + "/" + release + "/" + feature
	if _, err := os.Lstat(filepath.Join(clone, filepath.FromSlash(folder))); err == nil {
		return nil, fmt.Errorf("%s: %w; edit its files while they are draft or review, or reopen an approved spec with dashcrew spec revise %s", folder, ErrExists, folder)
	}
	var written []string
	title := strings.ToUpper(feature[:1]) + strings.ReplaceAll(feature[1:], "-", " ")
	fill := strings.NewReplacer("{{title}}", title, "{{feature}}", feature, "{{release}}", release)
	for i, template := range Templates {
		path := filepath.Join(clone, filepath.FromSlash(template.Path))
		data, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			if err := writeFile(path, template.Content); err != nil {
				return written, err
			}
			written = append(written, template.Path)
			data = []byte(template.Content)
		} else if err != nil {
			return written, err
		}
		text := setField(setField(fill.Replace(string(data)), "status", Draft), "rev", "1")
		target := folder + "/" + Files[i]
		if err := writeFile(filepath.Join(clone, filepath.FromSlash(target)), text); err != nil {
			return written, err
		}
		written = append(written, target)
	}
	return written, nil
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// setField sets the front matter line `key: value`, added before the closing `---` when the key is missing. Every
// other byte stays as it was.
func setField(text, key, value string) string {
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(text, newline)
	if len(lines) == 0 || lines[0] != "---" {
		return text
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			lines = append(lines[:i], append([]string{key + ": " + value}, lines[i:]...)...)
			break
		}
		if strings.HasPrefix(lines[i], key+":") {
			lines[i] = key + ": " + value
			break
		}
	}
	return strings.Join(lines, newline)
}

// Root splits path at its last `specs` element: the knowledge root before it and the slash-separated path from
// `specs` on; false when path has no such element.
func Root(path string) (root, relative string, ok bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", false
	}
	parts := strings.Split(filepath.ToSlash(abs), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == Dir {
			return filepath.FromSlash(strings.Join(parts[:i], "/")), strings.Join(parts[i:], "/"), true
		}
	}
	return "", "", false
}

// Display is path as commands print it: from `specs/` on when it is under one, else slash-separated as given.
func Display(path string) string {
	if _, relative, ok := Root(path); ok {
		return relative
	}
	return filepath.ToSlash(path)
}

// FilesUnder lists the spec files at path: path itself when it is a file, else every prd.md and trd.md below it,
// sorted.
func FilesUnder(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	var files []string
	err = filepath.WalkDir(path, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && contains(Files, entry.Name()) {
			files = append(files, file)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// FileResult is the lint of one file; Path is as Display prints it.
type FileResult struct {
	Path     string
	Problems []Problem
}

// LintFiles lints each file. With against, each is compared with the file at that git ref; a file the ref lacks
// counts as new. Without it, a file at rev 1 counts as new.
func LintFiles(files []string, against string) ([]FileResult, error) {
	var out []FileResult
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return out, err
		}
		doc := Parse(string(data))
		isNew := doc.Rev <= 1
		var base *Base
		if against != "" {
			text, found, err := AtRef(file, against)
			if err != nil {
				return out, err
			}
			isNew = !found
			if found {
				base = &Base{Doc: Parse(text), Text: strings.ReplaceAll(text, "\r\n", "\n"), Ref: against}
			}
		}
		out = append(out, FileResult{Path: Display(file), Problems: Lint(doc, base, isNew)})
	}
	return out, nil
}

// AtRef is file as the git ref holds it; false when the ref has no such file. A folder outside a repository or an
// unknown ref is an error.
func AtRef(file, ref string) (string, bool, error) {
	dir := filepath.Dir(file)
	if _, err := git(dir, "rev-parse", "--git-dir"); err != nil {
		return "", false, fmt.Errorf("%s is not inside a git repository", dir)
	}
	if _, err := git(dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil {
		return "", false, fmt.Errorf("unknown git ref %s", ref)
	}
	text, err := git(dir, "show", ref+":./"+filepath.Base(file))
	if err != nil {
		return "", false, nil
	}
	return text, true, nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	return stdout.String(), err
}

// Revised is what Revise changed: the knowledge root, the spec folder from `specs/` on, the files, their old
// revisions and the new one.
type Revised struct {
	Root, Folder string
	Files        []string
	From         []int
	Rev          int
}

// Revise sets every file of the spec at path (its folder or one of its files) from approved to review, with rev one
// past the highest of them. Nothing is written unless every file is approved. It never runs git.
func Revise(path string) (Revised, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Revised{}, fmt.Errorf("no such spec: %s", path)
	}
	folder := path
	if !info.IsDir() {
		folder = filepath.Dir(path)
	}
	root, relative, ok := Root(folder)
	if !ok || relative == Dir {
		return Revised{}, fmt.Errorf("%s is not a spec folder under %s/", path, Dir)
	}
	r := Revised{Root: root, Folder: relative}
	texts := map[string]string{}
	for _, name := range Files {
		file := filepath.Join(folder, name)
		data, err := os.ReadFile(file)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return Revised{}, err
		}
		doc := Parse(string(data))
		display := relative + "/" + name
		switch {
		case doc.Status != Approved:
			return Revised{}, fmt.Errorf("%s is %s, not approved; agents and members edit it directly", display, orUnknown(doc.Status))
		case doc.Rev < 1:
			return Revised{}, fmt.Errorf("%s has no valid rev; fix it first", display)
		}
		r.Files = append(r.Files, display)
		r.From = append(r.From, doc.Rev)
		r.Rev = max(r.Rev, doc.Rev+1)
		texts[file] = string(data)
	}
	if len(r.Files) == 0 {
		return Revised{}, fmt.Errorf("%s holds no %s", relative, strings.Join(Files, " or "))
	}
	for _, name := range Files {
		file := filepath.Join(folder, name)
		if text, ok := texts[file]; ok {
			if err := os.WriteFile(file, []byte(setField(setField(text, "status", Review), "rev", strconv.Itoa(r.Rev))), 0o644); err != nil {
				return r, err
			}
		}
	}
	return r, nil
}

func orUnknown(status string) string {
	if status == "" {
		return "without a status"
	}
	return status
}
