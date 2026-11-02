package knowledge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fahnaladitia/dashcrew/internal/spec"
)

// ErrNotEmpty is returned by Init for a folder that already holds something.
var ErrNotEmpty = errors.New("folder is not empty")

// InboxDir is the one folder of the knowledge repo agents may write to.
const InboxDir = "inbox"

const readme = `# Knowledge

Shared product knowledge for the team and its AI agents, kept as plain Markdown and reviewed like code.
dashcrew links a local clone of this repo into each project as ` + "`.ai/knowledge`" + `.

## Layout

- ` + "`features/<slug>.md`" + `: one feature: purpose, entry points per repo, state, endpoints, decisions and why, gotchas.
- ` + "`repos/<repo>.md`" + `: facts about one repo: setup, conventions not written elsewhere, traps.
- ` + "`inbox/<date>-<slug>.md`" + `: proposals from task retros, waiting for review.
- ` + "`specs/<release>/<feature>/prd.md` and `trd.md`" + `: the PRD and TRD of a large feature; ` + "`dashcrew spec new`" + ` starts one.
- ` + "`templates/`" + `: the page, PRD and TRD templates.

## Page format

- Front matter with ` + "`title`, `repos` and `tags`" + `.
- Every fact paragraph ends with ` + "`" + markerShape + "`" + `.
- Code pointers are written as ` + "`<repo>:<path>:<line>`" + `, in backticks, so tools can find them.

## Rules

- Agents write only under ` + "`inbox/`" + `, and under ` + "`specs/`" + ` to new files and files whose status is draft or review;
  the dashcrew guard blocks every other write into the clone.
- Pages in ` + "`features/` and `repos/`" + ` change through review and a pull request.
- Members open pull requests that add only ` + "`inbox/`" + ` files; one member at a time runs ` + "`dashcrew knowledge pull`" + `
  and ` + "`dashcrew knowledge review`" + ` on the main branch, so two reviews never edit the same page at once.
- A spec is approved by the merged pull request that sets ` + "`status: approved`" + `. It changes again only after a member
  runs ` + "`dashcrew spec revise`" + `.

## Pages

None reviewed yet.
`

const pageTemplate = `---
title: Feature name
repos: [repo-name]
tags: []
---

# Feature name

## Purpose

What the feature is for and who uses it. <!-- verified <repo>@<sha> <yyyy-mm-dd> -->

## Entry points

- ` + "`<repo>:<path>:<line>`" + `: what starts here.

## State and endpoints

## Decisions

Each decision with its reason.

## Gotchas
`

// scaffold is the layout Init writes, in the order it reports.
var scaffold = []struct{ path, content string }{
	{"README.md", readme},
	{"features/.gitkeep", ""},
	{"inbox/.gitkeep", ""},
	{"repos/.gitkeep", ""},
	{spec.Dir + "/.gitkeep", ""},
	{"templates/page.md", pageTemplate},
	{spec.Templates[0].Path, spec.Templates[0].Content},
	{spec.Templates[1].Path, spec.Templates[1].Content},
}

// Init writes a new knowledge repo layout into dir, which must be missing or empty; it returns the files written,
// slash-separated. It never runs git.
func Init(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	switch {
	case err == nil && !info.IsDir():
		return nil, fmt.Errorf("%s is not a folder", dir)
	case err == nil:
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		if len(entries) > 0 {
			return nil, fmt.Errorf("%s: %w; knowledge init only writes into a new or empty folder", dir, ErrNotEmpty)
		}
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	}
	written := []string{}
	for _, file := range scaffold {
		target := filepath.Join(dir, filepath.FromSlash(file.path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(target, []byte(file.content), 0o644); err != nil {
			return written, err
		}
		written = append(written, file.path)
	}
	return written, nil
}
