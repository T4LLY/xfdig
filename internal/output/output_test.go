package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/T4LLY/xfdig/internal/finder"
)

func TestJSONIncludesPRURLForCLIChaining(t *testing.T) {
	result := finder.Result{
		Query:      "deadlock",
		SearchType: "hybrid",
		Fixes: []finder.Fix{{
			Repo: "acme/tool", PR: 11, URL: "https://github.com/acme/tool/pull/11", Title: "fix",
		}},
	}
	var buf bytes.Buffer
	if err := JSON(&buf, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"url":"https://github.com/acme/tool/pull/11"`) {
		t.Fatalf("unexpected json: %s", buf.String())
	}
}
