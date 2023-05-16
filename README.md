# Installation

TODO - homebrew

It is recommended to alias `hg-go` to `hg` in your `.bashrc` or `.zshrc` for easier use.

# Usage

```
$ hggo
hg is set of commands for emulating a subset of Mercurial commands on a Git repository, as well as interacting with GitHub Pull Requests.

Usage:
  hg [command]

Available Commands:
  add         Alias of git add.
  amend       Commits changes as a new commit on the current branch and restacks descendant branches via merges (not rebases).
  bookmark    Bookmark (branch) management.
  cleanup     Cleanup merged branches and rebase their descendants on master.
  commit      Stage all files and commit.
  completion  Generate the autocompletion script for the specified shell
  diff        Alias of git diff.
  edit        Edits the branch description.
  help        Help about any command
  next        Checks out the child branch.
  prev        Checks out the parent branch.
  prsync      Syncs the local title and description to match the PR title and PR description.
  pull        Pull master from remote.
  rebase      Rebases the given branch and its descendants onto the given branch. Rebase is done with a merge instead of an actual rebase.
  revert      Revert file(s) to a given revision.
  smartlog    Displays a smartlog: a sparse graph of commits relevant to you.
  status      Alias of git status.
  submit      Submits GitHub Pull Requests for the current stack (current branch and its ancestors).
  update      Checkout the given rev. Rev can be a branch name or a commit hash. Snaps to a branch name if possible.

Flags:
  -h, --help   help for hg

Use "hg [command] --help" for more information about a command.
```

# Development

## Install golang

```
brew update && brew install golang
```

## Manage dependencies

Adding dependencies:

```
go get ...
```

Cleanup dependencies:

```
go mod tidy && go mod download
```

## Building

```
go build -o bin/hg-git main.go
```

## Running

```
go run main.go
```

## Formatter

Installation:

```
go install github.com/segmentio/golines@latest
```

## Release

Install goreleaser:

```
brew install goreleaser/tap/goreleaser
```

Local release:

```
goreleaser release --snapshot --skip-publish --clean
```

Release on GitHub will happen through GitHub Actions when pushing a new tag.

References:

* https://dev.to/aurelievache/learning-go-by-examples-part-8-automatically-cross-compile-release-your-go-app-457a
* https://dev.to/aurelievache/learning-go-by-examples-part-9-use-homebrew-goreleaser-for-distributing-a-golang-app-44ae