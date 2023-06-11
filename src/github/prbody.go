package github

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yapaluc/hg-git/src/util"
)

const previousComment = "<!-- previous -->"
const nextComment = "<!-- next -->"
const endPreambleComment = "<!-- end preamble -->"
const prevAndNextTableTemplate = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>%s | Current | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>%s |
| ------------- | ------------- | ------------- |
`
const defaultAnnotation = "&nbsp;"

type PrBody struct {
	PreviousPR string
	// TODO: support multiple "next"s
	NextPR      string
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
			prBody.NextPR = strings.TrimPrefix(
				strings.TrimSuffix(line, nextComment),
				"* **Next:** ",
			)
		} else if line == endPreambleComment {
			// If there is content before the end preamble comment but PreviousPR/NextPR were not populated,
			// this is not the right format.
			if i > 0 && prBody.PreviousPR == "" && prBody.NextPR == "" {
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
	prevAndNextTableRegex := fmt.Sprintf(prevAndNextTableTemplate, "(?P<prev>.*)", "(?P<next>.*)")
	prevAndNextTableRegex = strings.ReplaceAll(prevAndNextTableRegex, "|", `\|`)
	prevAndNextTableRegex = strings.Trim(prevAndNextTableRegex, "\n")
	r := regexp.MustCompile("(?s)" + prevAndNextTableRegex)
	match, err := util.RegexNamedMatches(r, rawTable)
	if err != nil {
		return nil, fmt.Errorf("parsing PR body to find PR urls: %w", err)
	}
	if match["prev"] != defaultAnnotation {
		prBody.PreviousPR = match["prev"]
	}
	if match["next"] != defaultAnnotation {
		prBody.NextPR = match["next"]
	}
	return &prBody, nil
}

func (p *PrBody) String() string {
	var prevAndNextTable string
	if p.PreviousPR != "" || p.NextPR != "" {
		previousAnnotation := defaultAnnotation
		if p.PreviousPR != "" {
			previousAnnotation = p.PreviousPR
		}
		nextAnnotation := defaultAnnotation
		if p.NextPR != "" {
			nextAnnotation = p.NextPR
		}
		prevAndNextTable = fmt.Sprintf(
			prevAndNextTableTemplate,
			previousAnnotation,
			nextAnnotation,
		)
	}
	return fmt.Sprintf("%s%s\n\n%s", prevAndNextTable, endPreambleComment, p.Description)
}
