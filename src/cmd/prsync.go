package cmd

import (
	"fmt"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/github"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newPrsyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prsync",
		Short: "Syncs the local title and description to match the PR title and PR description.",
		RunE:  runPrsync,
	}
}

func runPrsync(_ *cobra.Command, args []string) error {
	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	gh := github.New()
	prData, err := gh.FetchPRForBranch(currBranch)
	if err != nil {
		return fmt.Errorf("getting PR data for branch %q: %w", currBranch, err)
	}
	if prData == nil {
		return fmt.Errorf("no PR found for branch %q: %w", currBranch, err)
	}

	prBody, err := github.NewPrBody(prData.Body)
	if err != nil {
		return fmt.Errorf("parsing PR body for branch %q: %w", currBranch, err)
	}

	branchDesc := git.BranchDescription{
		Title: prData.Title,
		Body:  prBody.Description,
		PrURL: prData.URL,
	}
	err = writeBranchDescription(currBranch, branchDesc.String())
	if err != nil {
		return fmt.Errorf("updating the description for branch %q: %w", currBranch, err)
	}

	color.Green("Synced branch description from PR")
	return nil
}
