package github

import (
	"strings"
	"testing"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
)

const rawPrBodyWithPrevAndNextPR = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>PR_URL1 | Current | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>PR_URL2 |
| ------------- | ------------- | ------------- |
<!-- end preamble -->

content line 1
content line 2
`

const rawPrBodyWithPrevPR = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>PR_URL1 | Current | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>&nbsp; |
| ------------- | ------------- | ------------- |
<!-- end preamble -->

content line 1
content line 2
`

const rawPrBodyWithNextPR = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>&nbsp; | Current | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>PR_URL2 |
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
		nextPR      string
	}{
		"PR body with prev and next PR": {
			rawPrBody:   rawPrBodyWithPrevAndNextPR,
			description: "content line 1\ncontent line 2",
			previousPR:  "PR_URL1",
			nextPR:      "PR_URL2",
		},
		"PR body with prev PR": {
			rawPrBody:   rawPrBodyWithPrevPR,
			description: "content line 1\ncontent line 2",
			previousPR:  "PR_URL1",
			nextPR:      "",
		},
		"PR body with next PR": {
			rawPrBody:   rawPrBodyWithNextPR,
			description: "content line 1\ncontent line 2",
			previousPR:  "",
			nextPR:      "PR_URL2",
		},
		"PR body with no PRs": {
			rawPrBody:   rawPrBodyWithNoPRs,
			description: "content line 1\ncontent line 2",
			previousPR:  "",
			nextPR:      "",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			prBody, err := NewPrBody(strings.TrimSpace(tc.rawPrBody))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(prBody.Description).To(Equal(tc.description))
			g.Expect(prBody.PreviousPR).To(Equal(tc.previousPR))
			g.Expect(prBody.NextPR).To(Equal(tc.nextPR))
		})
	}
}

func TestPrBodyRoundtrip(t *testing.T) {
	testCases := map[string]PrBody{
		"PR body with prev and next PR": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  "PR_URL1",
			NextPR:      "PR_URL2",
		},
		"PR body with prev PR": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  "PR_URL1",
			NextPR:      "",
		},
		"PR body with next PR": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  "",
			NextPR:      "PR_URL2",
		},
		"PR body with no PRs": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  "",
			NextPR:      "",
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
			g.Expect(prBody.NextPR).To(Equal(tc.NextPR))
		})
	}
}
