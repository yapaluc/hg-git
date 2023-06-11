package github

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/yapaluc/hg-git/src/util"
)

const previousComment = "<!-- previous -->"
const nextComment = "<!-- next -->"
const endPreambleComment = "<!-- end preamble -->"
const prevAndNextTableTemplate = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>%s | Current%s | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>%s |
| ------------- | ------------- | ------------- |
`
const defaultAnnotation = "&nbsp;"

type PrBody struct {
	PreviousPR  string
	NextPRs     []string
	Description string
}

func NewPrBody(rawPrBody string) (*PrBody, error) {
	prBody1, err := newPrBody1(rawPrBody)
	if err == nil {
		return prBody1, nil
	}
	return newPrBody2(rawPrBody)
}

// Old format for backwards compatibility
func newPrBody1(rawPrBody string) (*PrBody, error) {
	var prBody PrBody
	lines := strings.Split(rawPrBody, "\n")
	for i, line := range lines {
		if strings.HasSuffix(line, previousComment) {
			prBody.PreviousPR = strings.TrimPrefix(
				strings.TrimSuffix(line, previousComment),
				"* **Previous:** ",
			)
		} else if strings.HasSuffix(line, nextComment) {
			prBody.NextPRs = []string{
				strings.TrimPrefix(
					strings.TrimSuffix(line, nextComment),
					"* **Next:** ",
				),
			}
		} else if line == endPreambleComment {
			// If there is content before the end preamble comment but PreviousPR/NextPR were not populated,
			// this is not the right format.
			if i > 0 && prBody.PreviousPR == "" && len(prBody.NextPRs) == 0 {
				return nil, fmt.Errorf("parsing PR body: %q", rawPrBody)
			}
			// There should be a blank line after the end preamble comment, but fail gracefully if not.
			if i+2 < len(lines) {
				prBody.Description = strings.Join(lines[i+2:], "\n")
				break
			}
		}
	}
	return &prBody, nil
}

// New format
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
		prBody.PreviousPR = prevParts[0]
	}
	if match["next"] != defaultAnnotation {
		prBody.NextPRs = strings.Split(match["next"], "<br>")
	}
	return &prBody, nil
}

func (p *PrBody) String() string {
	var prevAndNextTable string
	if p.PreviousPR != "" || len(p.NextPRs) > 0 {
		var previousPRAnnotation []string
		if p.PreviousPR != "" {
			previousPRAnnotation = append(previousPRAnnotation, p.PreviousPR)
		}

		var nextPRAnnotation []string
		if len(p.NextPRs) > 0 {
			sort.Slice(p.NextPRs, func(i, j int) bool {
				return PRNumFromPRURL(p.NextPRs[i]) < PRNumFromPRURL(p.NextPRs[j])
			})
			nextPRAnnotation = append(nextPRAnnotation, p.NextPRs...)
		} else {
			nextPRAnnotation = append(nextPRAnnotation, defaultAnnotation)
		}

		// Make sure previous annotation and current has the same number of lines as next annotation.
		for i := 0; i < len(nextPRAnnotation)-len(previousPRAnnotation); i++ {
			previousPRAnnotation = append(previousPRAnnotation, defaultAnnotation)
		}
		var currentAnnotation []string
		for i := 0; i < len(nextPRAnnotation); i++ {
			currentAnnotation = append(currentAnnotation, "<br>")
		}
		prevAndNextTable = fmt.Sprintf(
			prevAndNextTableTemplate,
			strings.Join(previousPRAnnotation, "<br>"),
			strings.Join(currentAnnotation, ""),
			strings.Join(nextPRAnnotation, "<br>"),
		)
	}
	return fmt.Sprintf("%s%s\n\n%s", prevAndNextTable, endPreambleComment, p.Description)
}
