package github

import (
	"fmt"
	"strings"
)

const previousComment = "<!-- previous -->"
const nextComment = "<!-- next -->"
const endPreambleComment = "<!-- end preamble -->"

type PrBody struct {
	PreviousPR  string
	NextPR      string
	Description string
}

func NewPrBody(rawPrBody string) (*PrBody, error) {
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
			// There should be a blank line after the end preamble comment, but fail gracefully if not.
			if i+2 < len(lines) {
				prBody.Description = strings.Join(lines[i+2:], "\n")
				break
			}
		}
	}
	return &prBody, nil
}

func (p *PrBody) String() string {
	// TODO: format these better, e.g., in a table?
	// TODO: support multiple "next"s
	var previousAnnotation string
	if p.PreviousPR != "" {
		previousAnnotation = fmt.Sprintf("* **Previous:** %s", p.PreviousPR)
	}
	var nextAnnotation string
	if p.NextPR != "" {
		nextAnnotation = fmt.Sprintf("* **Next:** %s", p.NextPR)
	}
	return fmt.Sprintf(
		"%s%s\n%s%s\n%s\n\n%s",
		previousAnnotation,
		previousComment,
		nextAnnotation,
		nextComment,
		endPreambleComment,
		p.Description,
	)
}
