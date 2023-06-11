package github

import (
	"strings"
	"testing"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
)

const rawPrBodyWithPrevAndNextPR = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>PR_URL1 | Current<br> | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>PR_URL2 |
| ------------- | ------------- | ------------- |
<!-- end preamble -->

content line 1
content line 2
`

const rawPrBodyWithMultipleNextPRs = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>PR_URL1<br>&nbsp; | Current<br><br> | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>PR_URL2<br>PR_URL3 |
| ------------- | ------------- | ------------- |
<!-- end preamble -->

content line 1
content line 2
`

const rawPrBodyWithPrevPR = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>PR_URL1 | Current<br> | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>&nbsp; |
| ------------- | ------------- | ------------- |
<!-- end preamble -->

content line 1
content line 2
`

const rawPrBodyWithNextPR = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>&nbsp; | Current<br> | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>PR_URL2 |
| ------------- | ------------- | ------------- |
<!-- end preamble -->

content line 1
content line 2
`

const rawPrBodyWithNoPRs = `
<!-- end preamble -->

content line 1
content line 2
`

func TestNewPrBody(t *testing.T) {
	testCases := map[string]struct {
		rawPrBody   string
		description string
		previousPR  string
		nextPRs     []string
	}{
		"PR body with prev and next PR": {
			rawPrBody:   rawPrBodyWithPrevAndNextPR,
			description: "content line 1\ncontent line 2",
			previousPR:  "PR_URL1",
			nextPRs:     []string{"PR_URL2"},
		},
		"PR body with multiple next PRs": {
			rawPrBody:   rawPrBodyWithMultipleNextPRs,
			description: "content line 1\ncontent line 2",
			previousPR:  "PR_URL1",
			nextPRs:     []string{"PR_URL2", "PR_URL3"},
		},
		"PR body with prev PR": {
			rawPrBody:   rawPrBodyWithPrevPR,
			description: "content line 1\ncontent line 2",
			previousPR:  "PR_URL1",
			nextPRs:     nil,
		},
		"PR body with next PR": {
			rawPrBody:   rawPrBodyWithNextPR,
			description: "content line 1\ncontent line 2",
			previousPR:  "",
			nextPRs:     []string{"PR_URL2"},
		},
		"PR body with no PRs": {
			rawPrBody:   rawPrBodyWithNoPRs,
			description: "content line 1\ncontent line 2",
			previousPR:  "",
			nextPRs:     nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			prBody, err := NewPrBody(strings.TrimSpace(tc.rawPrBody))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(prBody.Description).To(Equal(tc.description))
			g.Expect(prBody.PreviousPR).To(Equal(tc.previousPR))
			g.Expect(prBody.NextPRs).To(ConsistOf(tc.nextPRs))
		})
	}
}

func TestPrBodyRoundtrip(t *testing.T) {
	testCases := map[string]PrBody{
		"PR body with prev and next PR": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  "PR_URL1",
			NextPRs:     []string{"PR_URL2"},
		},
		"PR body with multiple next PRs": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  "PR_URL1",
			NextPRs:     []string{"PR_URL3", "PR_URL2"},
		},
		"PR body with prev PR": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  "PR_URL1",
			NextPRs:     nil,
		},
		"PR body with next PR": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  "",
			NextPRs:     []string{"PR_URL2"},
		},
		"PR body with no PRs": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  "",
			NextPRs:     nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			rawPrBody := tc.String()
			prBody, err := NewPrBody(rawPrBody)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(prBody.Description).To(Equal(tc.Description))
			g.Expect(prBody.PreviousPR).To(Equal(tc.PreviousPR))
			g.Expect(prBody.NextPRs).To(ConsistOf(tc.NextPRs))
		})
	}
}
