package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/yapaluc/hg-git/src/git"
	"github.com/yapaluc/hg-git/src/github"
	"github.com/yapaluc/hg-git/src/shell"
	"github.com/yapaluc/hg-git/src/util"

	"github.com/alessio/shellescape"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newSubmitCmd() *cobra.Command {
	var draft bool
	var cmd = &cobra.Command{
		Use:   "submit [-n draft]",
		Short: "Submits GitHub Pull Requests for the current stack (current branch and its ancestors).",
		RunE: func(_ *cobra.Command, args []string) error {
			return runSubmit(submitCfg{
				draft: draft,
				gh:    github.New(),
			})
		},
	}
	cmd.Flags().BoolVarP(&draft, "draft", "n", false, "Create Pull Request as a draft")
	return cmd
}

type submitCfg struct {
	draft           bool
	gh              *github.GitHub
	gitMasterBranch string
}

func runSubmit(cfg submitCfg) error {
	repoData, err := git.NewRepoData()
	if err != nil {
		return err
	}
	cfg.gitMasterBranch = repoData.MasterBranch

	currBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}

	node, ok := repoData.BranchNameToNode[currBranch]
	if !ok {
		return fmt.Errorf("missing node for branch %q", currBranch)
	}

	stack, err := getStack(node)
	if err != nil {
		return fmt.Errorf("getting stack: %w", err)
	}

	// Process branches in reverse, starting from the root
	for i := len(stack) - 1; i >= 0; i-- {
		err := processBranch(cfg, stack[i])
		if err != nil {
			return fmt.Errorf("processing branch %s: %w", stack[i].branchName, err)
		}
	}

	return nil
}

type stackEntry struct {
	branchName string
	node       *git.TreeNode
}

func getStack(node *git.TreeNode) ([]*stackEntry, error) {
	var stack []*stackEntry
	for node != nil {
		if node.CommitMetadata.IsEffectiveMaster() {
			break
		}
		branchNames := node.CommitMetadata.CleanedBranchNames()
		if len(branchNames) > 1 {
			return nil, fmt.Errorf(
				"multiple branch names for branch %q - there should only be one branch name for no ambiguity in pushing to remote",
				node.CommitMetadata.ShortCommitHash,
			)
		}
		stack = append(stack, &stackEntry{
			branchName: branchNames[0],
			node:       node,
		})
		node = node.BranchParent
	}
	return stack, nil
}

func processBranch(cfg submitCfg, stackEntry *stackEntry) error {
	sp := spinner.New(
		spinner.CharSets[9],
		100*time.Millisecond,
		spinner.WithColor("reset"),
	)
	prefix := color.YellowString("%s: ", stackEntry.branchName)
	sp.Prefix = prefix
	sp.Suffix = color.YellowString(" processing")
	sp.Start()

	wasPushed, err := pushBranch(stackEntry.branchName, sp)
	if err != nil {
		return fmt.Errorf("pushing branch %q: %w", stackEntry.branchName, err)
	}

	prURL, prStatus, err := createOrUpdatePR(cfg, stackEntry, sp)
	if err != nil {
		return fmt.Errorf("creating or updating PR for %q: %w", stackEntry.branchName, err)
	}

	var finalStatus status
	if wasPushed {
		switch prStatus {
		case statusCreated:
			finalStatus = statusCreated
		case statusUpdated, statusSkipped:
			finalStatus = statusUpdated
		}
	} else {
		finalStatus = prStatus
	}

	prLink := util.Linkify(github.PRStrFromPRURL(prURL), prURL)
	sp.FinalMSG = prefix + fmt.Sprintf(
		"%s (%s)\n",
		color.New(color.Bold).Sprint(prLink),
		finalStatus.String(),
	)
	sp.Stop()
	return nil
}

func pushBranch(branchName string, sp *spinner.Spinner) (bool, error) {
	sp.Suffix = " pushing to remote"
	out, err := shell.Run(
		shell.Opt{CombinedStdoutStderrOutput: true},
		fmt.Sprintf("git push origin %s", shellescape.Quote(branchName)),
	)
	if err != nil {
		return false, fmt.Errorf("running git push: %w", err)
	}

	if strings.Contains(out, "Everything up-to-date") {
		return false, nil
	}
	return true, nil
}

type status struct {
	slug string
}

func (s status) String() string {
	return s.slug
}

var statusUnknown = status{""}
var statusCreated = status{"created"}
var statusSkipped = status{"skipped"}
var statusUpdated = status{"updated"}

func createOrUpdatePR(
	cfg submitCfg,
	stackEntry *stackEntry,
	sp *spinner.Spinner,
) (string, status, error) {
	parent := stackEntry.node.BranchParent
	if parent == nil {
		return "", statusUnknown, fmt.Errorf("no parent found for branch %q", stackEntry.branchName)
	}

	// Validate parent branch.
	var parentPRData *github.PullRequest
	if !parent.CommitMetadata.IsEffectiveMaster() {
		sp.Suffix = " fetching parent PR"
		parentBranch := parent.CommitMetadata.CleanedBranchNames()[0]
		parentPRDataLocal, err := cfg.gh.FetchPRForBranch(parentBranch)
		parentPRData = parentPRDataLocal
		if err != nil {
			return "", statusUnknown, fmt.Errorf(
				"fetching PR data for parent branch of %q (%q): %w",
				stackEntry.branchName,
				parentBranch,
				err,
			)
		}
		if parentPRData == nil {
			return "", statusUnknown, fmt.Errorf(
				"parent branch of %q (%q) does not have a PR: %w",
				stackEntry.branchName,
				parentBranch,
				err,
			)
		}
	}

	sp.Suffix = " fetching current PR"
	prData, err := cfg.gh.FetchPRForBranch(stackEntry.branchName)
	if err != nil {
		return "", statusUnknown, fmt.Errorf(
			"fetching PR data for branch %q: %w",
			stackEntry.branchName,
			err,
		)
	}

	var prURL string
	var status status
	if prData == nil {
		prURL, status, err = createPR(cfg, stackEntry, parentPRData, sp)
		if err != nil {
			return "", statusUnknown, fmt.Errorf(
				"creating PR for branch %q: %w",
				stackEntry.branchName,
				err,
			)
		}
	} else {
		prURL, status, err = updatePR(cfg, stackEntry, prData, parentPRData, sp)
		if err != nil {
			return "", statusUnknown, fmt.Errorf("updating PR for branch %q: %w", stackEntry.branchName, err)
		}
	}

	err = updateNextInParentPR(prURL, parentPRData, sp)
	if err != nil {
		return "", statusUnknown, fmt.Errorf(
			"updating next in parent PR of %q: %w",
			stackEntry.branchName,
			err,
		)
	}

	err = addPRURLToBranchDescription(stackEntry, prURL, sp)
	if err != nil {
		return "", statusUnknown, fmt.Errorf(
			"adding PR URL to branch description of branch %q: %w",
			stackEntry.branchName,
			err,
		)
	}

	return prURL, status, nil
}

func createPR(
	cfg submitCfg,
	stackEntry *stackEntry,
	parentPRData *github.PullRequest,
	sp *spinner.Spinner,
) (string, status, error) {
	sp.Suffix = " creating PR"

	commitMetadata := stackEntry.node.CommitMetadata
	if commitMetadata.BranchDescription == nil {
		return "", statusUnknown, fmt.Errorf(
			"no branch description for branch %q - add one by running the subcommand \"edit\"",
			stackEntry.branchName,
		)
	}

	var parentPRURL string
	if parentPRData != nil {
		parentPRURL = parentPRData.URL
	}
	prBody := github.PrBody{
		PreviousPR:  parentPRURL,
		Description: commitMetadata.BranchDescription.Body,
	}
	args := []string{
		"--head",
		stackEntry.branchName,
		"--title",
		shellescape.Quote(commitMetadata.BranchDescription.Title),
		"--body",
		shellescape.Quote(prBody.String()),
	}
	if parentPRData != nil {
		args = append(args, "--base")
		args = append(args, parentPRData.HeadRefName)
	}
	if cfg.draft {
		args = append(args, "--draft")
	}

	prURL, err := shell.Run(
		shell.Opt{},
		fmt.Sprintf("gh pr create %s", strings.Join(args, " ")),
	)
	if err != nil {
		return "", statusUnknown, fmt.Errorf(
			"creating PR for branch %q: %w",
			stackEntry.branchName,
			err,
		)
	}

	return prURL, statusCreated, nil
}

func updatePR(
	cfg submitCfg,
	stackEntry *stackEntry,
	prData *github.PullRequest,
	parentPRData *github.PullRequest,
	sp *spinner.Spinner,
) (string, status, error) {
	var args []string
	commitMetadata := stackEntry.node.CommitMetadata
	if commitMetadata.BranchDescription == nil {
		return "", statusUnknown, fmt.Errorf(
			"no branch description for branch %q - add one by running the subcommand \"edit\"",
			stackEntry.branchName,
		)
	}
	var parentBranch string
	if parentPRData == nil {
		parentBranch = cfg.gitMasterBranch
	} else {
		parentBranch = parentPRData.HeadRefName
	}
	if parentBranch != prData.BaseRefName {
		args = append(args, "--base")
		args = append(args, shellescape.Quote(parentBranch))
	}
	if commitMetadata.BranchDescription.Title != prData.Title {
		args = append(args, "--title")
		args = append(args, shellescape.Quote(commitMetadata.BranchDescription.Title))
	}
	updatedPRBody, err := getUpdatedPRBody(cfg, stackEntry, prData, parentBranch, parentPRData)
	if err != nil {
		return "", statusUnknown, fmt.Errorf(
			"getting updated PR body for branch %q: %w",
			stackEntry.branchName,
			err,
		)
	}
	if updatedPRBody != prData.Body {
		args = append(args, "--body")
		args = append(args, shellescape.Quote(updatedPRBody))
	}

	if len(args) == 0 {
		return prData.URL, statusSkipped, nil
	}

	sp.Suffix = " updating PR fields"
	_, err = shell.Run(
		shell.Opt{},
		fmt.Sprintf(
			"gh pr edit %s %s",
			shellescape.Quote(stackEntry.branchName),
			strings.Join(args, " "),
		),
	)
	if err != nil {
		return "", statusUnknown, fmt.Errorf(
			"editing PR for branch %q: %w",
			stackEntry.branchName,
			err,
		)
	}
	return prData.URL, statusUpdated, nil
}

func getUpdatedPRBody(
	cfg submitCfg,
	stackEntry *stackEntry,
	prData *github.PullRequest,
	parentBranch string,
	parentPRData *github.PullRequest,
) (string, error) {
	prBody, err := github.NewPrBody(prData.Body)
	if err != nil {
		return "", fmt.Errorf("parsing PR body of branch %q: %w", stackEntry.branchName, err)
	}

	prBody.Description = stackEntry.node.CommitMetadata.BranchDescription.Body

	// If previous PR was merged, keep it. Else, replace it with the new previous URL.
	var newPreviousPR string
	if parentPRData != nil {
		newPreviousPR = parentPRData.URL
	}
	if prBody.PreviousPR != "" {
		previousPRData, err := cfg.gh.FetchPRByURL(prBody.PreviousPR)
		if err != nil {
			return "", fmt.Errorf("fetching previous PR at URL %q: %w", prBody.PreviousPR, err)
		}
		if previousPRData.State == "MERGED" {
			newPreviousPR = prBody.PreviousPR
		}
	}
	prBody.PreviousPR = newPreviousPR

	// Remove the "Next" annotation if its base branch is not pointing to this branch.
	if prBody.NextPR != "" {
		nextPRData, err := cfg.gh.FetchPRByURL(prBody.NextPR)
		if err != nil {
			return "", fmt.Errorf("fetching next PR at URL %q: %w", prBody.NextPR, err)
		}
		if nextPRData.BaseRefName != stackEntry.branchName {
			prBody.NextPR = ""
		}
	}

	return prBody.String(), nil
}

func updateNextInParentPR(
	prURL string,
	parentPRData *github.PullRequest,
	sp *spinner.Spinner,
) error {
	if parentPRData == nil {
		return nil
	}

	parentPrBody, err := github.NewPrBody(parentPRData.Body)
	if err != nil {
		return fmt.Errorf("getting PR body of PR URL %q: %w", parentPRData.URL, err)
	}
	if parentPrBody.NextPR == prURL {
		return nil
	}
	parentPrBody.NextPR = prURL

	sp.Suffix = " updating parent PR with forward reference"
	_, err = shell.Run(
		shell.Opt{},
		fmt.Sprintf(
			"gh pr edit %s --body %s",
			shellescape.Quote(parentPRData.URL),
			shellescape.Quote(parentPrBody.String()),
		),
	)
	if err != nil {
		return fmt.Errorf("editing parent PR %q to reference %q: %w", parentPRData.URL, prURL, err)
	}

	return nil
}

func addPRURLToBranchDescription(
	stackEntry *stackEntry,
	prURL string,
	sp *spinner.Spinner,
) error {
	sp.Suffix = " adding PR URL to local branch description"

	branchDescription := stackEntry.node.CommitMetadata.BranchDescription
	branchDescription.PrURL = prURL
	err := writeBranchDescription(stackEntry.branchName, branchDescription.String())
	if err != nil {
		return fmt.Errorf(
			"writing branch description for branch %q: %w",
			stackEntry.branchName,
			err,
		)
	}
	return nil
}
