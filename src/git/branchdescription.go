package git

import (
	"fmt"
	"strings"
)

type BranchDescription struct {
	Title string
	Body  string
	PrURL string
}

func NewBranchDescription(firstLine string, remainingLines []string) *BranchDescription {
	title := strings.TrimSpace(firstLine)
	body := strings.TrimSpace(strings.Join(remainingLines, "\n"))
	bodyLines := strings.Split(body, "\n")
	var prURL string
	var finalBody string
	if strings.HasPrefix(bodyLines[len(bodyLines)-1], "PR: ") {
		prURL = strings.TrimPrefix(bodyLines[len(bodyLines)-1], "PR: ")
		bodyLines = bodyLines[:len(bodyLines)-1]
		finalBody = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	} else {
		finalBody = strings.Join(bodyLines, "\n")
	}
	return &BranchDescription{
		Title: title,
		Body:  finalBody,
		PrURL: prURL,
	}
}

func (b *BranchDescription) String() string {
	desc := fmt.Sprintf("%s\n%s", b.Title, b.Body)
	if b.PrURL != "" {
		desc += fmt.Sprintf("\n\nPR: %s\n\n", b.PrURL)
	}
	return desc
}
