package github

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/samber/lo"
	"github.com/yapaluc/hg-git/src/util"
)

const previousComment = "<!-- previous -->"
const nextComment = "<!-- next -->"
const endPreambleComment = "<!-- end preamble -->"
const prevAndNextTableTemplate = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>%s | Current%s | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>%s |
| ------------- | ------------- | ------------- |
`
const stackIndicatorTemplate = `> [!NOTE]
> This PR is part of a stack:
%s

`

const defaultAnnotation = "&nbsp;"

const nonBreakingSpace = "\u00A0"

type PrBody struct {
	PreviousPR  int
	NextPRs     []int
	Description string
}

func NewPrBody(rawPrBody string) (*PrBody, error) {
	prBody1, err := newPrBody1(rawPrBody)
	if err == nil {
		return prBody1, nil
	}

	prBody2, err := newPrBody2(rawPrBody)
	if err == nil {
		return prBody2, nil
	}

	prBody3, err := newPrBody3(rawPrBody)
	if err == nil {
		return prBody3, nil
	}

	// Must be a stackless PR. Assume the entire body is the description.
	return &PrBody{Description: rawPrBody}, nil
}

// Old format for backwards compatibility
func newPrBody1(rawPrBody string) (*PrBody, error) {
	var prBody PrBody
	lines := strings.Split(rawPrBody, "\n")
	for i, line := range lines {
		if strings.HasSuffix(line, previousComment) {
			rawPreviousPR := strings.TrimPrefix(
				strings.TrimSuffix(line, previousComment),
				"* **Previous:** ",
			)
			prBody.PreviousPR = PRNumFromNumOrURL(rawPreviousPR)
		} else if strings.HasSuffix(line, nextComment) {
			rawNextPR := strings.TrimPrefix(
				strings.TrimSuffix(line, nextComment),
				"* **Next:** ",
			)
			prBody.NextPRs = []int{
				PRNumFromNumOrURL(rawNextPR),
			}
		} else if line == endPreambleComment {
			// There should be a blank line after the end preamble comment, but fail gracefully if not.
			if i+2 < len(lines) {
				prBody.Description = strings.Join(lines[i+2:], "\n")
				break
			}
		}
	}

	// If PreviousPR/NextPR were not populated, return an error to allow top-level handling of stackless PRs.
	if prBody.PreviousPR == 0 && len(prBody.NextPRs) == 0 {
		return nil, fmt.Errorf("parsing PR body: %q", rawPrBody)
	}

	return &prBody, nil
}

// Old format for backwards compatibility
func newPrBody2(rawPrBody string) (*PrBody, error) {
	lines := strings.Split(rawPrBody, "\n")
	var endPreambleIndex *int
	for i, line := range lines {
		if line == endPreambleComment {
			endPreambleIndex = &i
			break
		}
	}
	if endPreambleIndex == nil {
		return nil, fmt.Errorf("could not find end preamble line in PR body: %q", rawPrBody)
	}

	var prBody PrBody

	// There should be a blank line after the end preamble comment, but fail gracefully if not.
	if *endPreambleIndex+2 < len(lines) {
		prBody.Description = strings.Join(lines[*endPreambleIndex+2:], "\n")
	}

	// Parse the previous/next PR.
	rawTable := strings.Join(lines[:*endPreambleIndex], "\n")
	rawTable = strings.TrimSpace(rawTable)
	if rawTable == "" {
		return &prBody, nil
	}
	prevAndNextTableRegex := fmt.Sprintf(
		prevAndNextTableTemplate,
		"(?P<prev>.*)",
		"(<br>)*",
		"(?P<next>.*)",
	)
	prevAndNextTableRegex = strings.ReplaceAll(prevAndNextTableRegex, "|", `\|`)
	prevAndNextTableRegex = strings.Trim(prevAndNextTableRegex, "\n")
	r := regexp.MustCompile("(?s)" + prevAndNextTableRegex)
	match, err := util.RegexNamedMatches(r, rawTable)
	if err != nil {
		return nil, fmt.Errorf("parsing PR body to find PR urls: %w", err)
	}
	prevParts := strings.Split(match["prev"], "<br>")
	if prevParts[0] != defaultAnnotation {
		prBody.PreviousPR = PRNumFromNumOrURL(prevParts[0])
	}
	if match["next"] != defaultAnnotation {
		prBody.NextPRs = lo.Map(
			strings.Split(match["next"], "<br>"),
			func(n string, _ int) int {
				return PRNumFromNumOrURL(n)
			},
		)
	}

	// If PreviousPR/NextPR were not populated, return an error to allow top-level handling of stackless PRs.
	if prBody.PreviousPR == 0 && len(prBody.NextPRs) == 0 {
		return nil, fmt.Errorf("parsing PR body: %q", rawPrBody)
	}

	return &prBody, nil
}

// New format
func newPrBody3(rawPrBody string) (*PrBody, error) {
	var prBody PrBody
	rowPattern := regexp.MustCompile(
		`^> \|\s*(?P<prev>.*?)\s*\|\s*\*This PR\*\s*\|\s*(?P<next>.*?)\s*\|$`,
	)
	var rowIndex int

	lines := strings.Split(rawPrBody, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Parse the table row.
		match, err := util.RegexNamedMatches(rowPattern, line)
		if err != nil {
			return nil, fmt.Errorf("parsing PR body to find PR stack table: %w", err)
		}
		if match != nil {
			prBody.PreviousPR = PRNumFromNumOrURL(strings.TrimSpace(match["prev"]))
			nextRaw := strings.TrimSpace(match["next"])
			if nextRaw != "" {
				prBody.NextPRs = lo.Map(
					strings.Split(nextRaw, ", "),
					func(n string, _ int) int {
						return PRNumFromNumOrURL(n)
					},
				)
			}
			rowIndex = i
			break
		}
	}

	// If stack block was not found, return an error to allow top-level handling of stackless PRs.
	if prBody.PreviousPR == 0 && len(prBody.NextPRs) == 0 {
		return nil, fmt.Errorf("parsing PR body: %q", rawPrBody)
	}

	// If stack block was found, everything after is the description.
	prBody.Description = strings.TrimSpace(strings.Join(lines[rowIndex+1:], "\n"))

	return &prBody, nil
}

func (p *PrBody) toPRStackTable() string {
	if p.PreviousPR == 0 && len(p.NextPRs) == 0 {
		return ""
	}

	// Sort NextPRs to make the output deterministic.
	sort.Ints(p.NextPRs)

	// Render cells.
	var previousCell string
	if p.PreviousPR != 0 {
		previousCell = PRStrFromPRNum(p.PreviousPR)
	}
	var nextCell string
	if len(p.NextPRs) != 0 {
		nextCell = strings.Join(
			lo.Map(p.NextPRs, func(n int, _ int) string {
				return PRStrFromPRNum(n)
			}),
			", ",
		)
	}

	// Render table.
	table := generateSingleRowMarkdownTable(
		[]string{"← Previous", "⬤ Current", "→ Next"},
		[]string{previousCell, "*This PR*", nextCell},
		"> ",
	)

	// Render.
	return fmt.Sprintf(stackIndicatorTemplate, table)
}

func (p *PrBody) ToMarkdown() string {
	return fmt.Sprintf("%s%s", p.toPRStackTable(), p.Description)
}

// generateSingleRowMarkdownTable builds a table that renders such that the outer columns are equal-width,
// determined by max width of those columns. The middle column is rendered naturally.
// It also attempts best affort to align the raw Markdown to be human readable.
func generateSingleRowMarkdownTable(headers []string, row []string, prefix string) string {
	if len(headers) != 3 || len(row) != 3 {
		return fmt.Sprintf("expected exactly 3 columns but got %d", len(headers))
	}

	// Determine fixed width for outer columns
	maxOuter := lo.Max([]int{
		runewidth.StringWidth(headers[0]),
		runewidth.StringWidth(row[0]),
		runewidth.StringWidth(headers[2]),
		runewidth.StringWidth(row[2]),
	})

	// Actual width of middle column (2nd col)
	midWidth := lo.Max([]int{runewidth.StringWidth(headers[1]), runewidth.StringWidth(row[1])})

	padWithNonVisibleSpacesForRawReadability := func(content string, width int) string {
		visibleLen := len(content)
		if visibleLen >= width {
			return content
		}
		return content + strings.Repeat(" ", width-visibleLen)
	}
	centerPadWithNBSPForRenderedReadability := func(content string, width int) string {
		visibleLen := len(content)
		if visibleLen >= width {
			return content
		}
		if visibleLen == 0 {
			// No need to use NBSPs when the cell is empty and its width is determined by another cell in the column.
			return padWithNonVisibleSpacesForRawReadability(content, width)
		}
		// Scale up the width to account for GitHub's non-monospace rendering.
		width = int(
			float64(width) * 1.5,
		)
		padding := width - visibleLen
		left := padding / 2
		right := padding - left
		return strings.Repeat(
			nonBreakingSpace,
			left,
		) + content + strings.Repeat(
			nonBreakingSpace,
			right,
		)
	}

	var markdownLines []string

	// Header row
	markdownLines = append(markdownLines, fmt.Sprintf(
		"%s| %s | %s | %s |",
		prefix,
		padWithNonVisibleSpacesForRawReadability(headers[0], maxOuter),
		padWithNonVisibleSpacesForRawReadability(headers[1], midWidth),
		padWithNonVisibleSpacesForRawReadability(headers[2], maxOuter),
	))

	// Divider row
	markdownLines = append(markdownLines, fmt.Sprintf(
		"%s| %s | %s | %s |",
		prefix,
		strings.Repeat("-", maxOuter),
		strings.Repeat("-", midWidth),
		strings.Repeat("-", maxOuter),
	))

	// Data row
	markdownLines = append(markdownLines, fmt.Sprintf(
		"%s| %s | %s | %s |",
		prefix,
		centerPadWithNBSPForRenderedReadability(row[0], maxOuter),
		padWithNonVisibleSpacesForRawReadability(row[1], midWidth),
		centerPadWithNBSPForRenderedReadability(row[2], maxOuter),
	))

	return strings.Join(markdownLines, "\n")
}
