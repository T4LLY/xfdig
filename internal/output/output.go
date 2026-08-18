package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/T4LLY/xfdig/internal/finder"
)

func JSON(w io.Writer, result finder.Result) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(result)
}

func Text(w io.Writer, result finder.Result) error {
	fmt.Fprintf(w, "status: %s\nquery: %s\nlanguage: %s\n", result.Status, result.Query, result.Language)
	if result.SearchType != "" {
		fmt.Fprintf(w, "search: %s\n", result.SearchType)
	}
	if result.Error != "" {
		fmt.Fprintf(w, "error: %s\n", result.Error)
	}
	if result.Since != "" || result.Until != "" {
		since := result.Since
		until := result.Until
		if since == "" {
			since = "*"
		}
		if until == "" {
			until = "*"
		}
		fmt.Fprintf(w, "closed: %s .. %s\n", since, until)
	}
	if result.Status != finder.StatusFailure && len(result.Fixes) == 0 {
		fmt.Fprintln(w, "no merged linked fixes found")
	}
	for i, fix := range result.Fixes {
		fmt.Fprintf(w, "\n%d. %s#%d  %s\n", i+1, fix.Repo, fix.PR, fix.Title)
		fmt.Fprintf(w, "   %s\n", fix.URL)
		fmt.Fprintf(w, "   issue: %s#%d (rank %d)\n", fix.Issue.Repo, fix.Issue.Number, fix.Evidence.IssueRank)
		fmt.Fprint(w, "   evidence: closed, linked, merged")
		if len(fix.Evidence.MatchedTerms) > 0 {
			fmt.Fprintf(w, "; terms=%s", strings.Join(fix.Evidence.MatchedTerms, ","))
		}
		fmt.Fprintln(w)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warning)
	}
	return nil
}
