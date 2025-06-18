package github

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

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
const prevAndNextTableTemplate2 = `> [!NOTE]
> This PR is part of a stack:
> | ⬅️ Previous | 🔵 Current | ➡️ Next |
> | ----------- | --------- | ------ |
> | %s | *This PR* | %s |

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

	// Pad columns with non-breaking spaces to make the column widths even.
	targetColWidth := lo.Max([]int{
		len("⬅️ Previous"), // Previous is the longer column name of the two
		len(previousCell),
		len(nextCell),
	})
	previousCellPadding := strings.Repeat(nonBreakingSpace, (targetColWidth-len(previousCell))/2)
	previousCell = previousCellPadding + previousCell + previousCellPadding
	nextCellPadding := strings.Repeat(nonBreakingSpace, (targetColWidth-len(nextCell))/2)
	nextCell = nextCellPadding + nextCell + nextCellPadding

	// Render table.
	return fmt.Sprintf(prevAndNextTableTemplate2, previousCell, nextCell)
}

func (p *PrBody) ToMarkdown() string {
	return fmt.Sprintf("%s%s", p.toPRStackTable(), p.Description)
}
