package github

import (
	"strings"
	"testing"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
)

const rawPrBodyWithPrevAndNextPR = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>#1 | Current<br> | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>#2 |
| ------------- | ------------- | ------------- |
<!-- end preamble -->

content line 1
content line 2
`

const rawPrBodyWithMultipleNextPRs = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>#1<br>&nbsp; | Current<br><br> | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>#2<br>#3 |
| ------------- | ------------- | ------------- |
<!-- end preamble -->

content line 1
content line 2
`

const rawPrBodyWithPrevPR = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>#1 | Current<br> | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>&nbsp; |
| ------------- | ------------- | ------------- |
<!-- end preamble -->

content line 1
content line 2
`

const rawPrBodyWithNextPR = `
| ◀<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Previous&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>&nbsp; | Current<br> | ▶<br>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Next&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<br>#2 |
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

func TestNewPrBody_newPrBody2(t *testing.T) {
	testCases := map[string]struct {
		rawPrBody   string
		description string
		previousPR  int
		nextPRs     []int
	}{
		"PR body with prev and next PR": {
			rawPrBody:   rawPrBodyWithPrevAndNextPR,
			description: "content line 1\ncontent line 2",
			previousPR:  1,
			nextPRs:     []int{2},
		},
		"PR body with multiple next PRs": {
			rawPrBody:   rawPrBodyWithMultipleNextPRs,
			description: "content line 1\ncontent line 2",
			previousPR:  1,
			nextPRs:     []int{2, 3},
		},
		"PR body with prev PR": {
			rawPrBody:   rawPrBodyWithPrevPR,
			description: "content line 1\ncontent line 2",
			previousPR:  1,
			nextPRs:     nil,
		},
		"PR body with next PR": {
			rawPrBody:   rawPrBodyWithNextPR,
			description: "content line 1\ncontent line 2",
			nextPRs:     []int{2},
		},
		"PR body with no PRs": {
			rawPrBody:   rawPrBodyWithNoPRs,
			description: "content line 1\ncontent line 2",
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
			PreviousPR:  1,
			NextPRs:     []int{2},
		},
		"PR body with multiple next PRs": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  1,
			NextPRs:     []int{2, 3},
		},
		"PR body with prev PR": {
			Description: "content line 1\ncontent line 2",
			PreviousPR:  1,
			NextPRs:     nil,
		},
		"PR body with next PR": {
			Description: "content line 1\ncontent line 2",
			NextPRs:     []int{2},
		},
		"PR body with no PRs": {
			Description: "content line 1\ncontent line 2",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			rawPrBody := tc.ToMarkdown()
			prBody, err := NewPrBody(rawPrBody)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(prBody.Description).To(Equal(tc.Description))
			g.Expect(prBody.PreviousPR).To(Equal(tc.PreviousPR))
			g.Expect(prBody.NextPRs).To(ConsistOf(tc.NextPRs))
		})
	}
}
