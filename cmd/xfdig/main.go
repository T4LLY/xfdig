package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/T4LLY/xfdig/internal/finder"
	gh "github.com/T4LLY/xfdig/internal/github"
	"github.com/T4LLY/xfdig/internal/output"
)

const version = "0.1.0"

func main() {
	os.Exit(run())
}

func run() int {
	fs := flag.NewFlagSet("xfdig", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	limit := fs.Int("n", 5, "maximum number of fixes")
	text := fs.Bool("t", false, "human-readable output")
	showVersion := fs.Bool("version", false, "print version")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: xfdig [-n N] [-t] <bug, error, or symptom>")
		fmt.Fprintln(os.Stderr, "\nFind merged GitHub PRs linked to similar closed issues.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Println("xfdig " + version)
		return 0
	}
	if *limit < 1 || *limit > 20 {
		fmt.Fprintln(os.Stderr, "xfdig: -n must be between 1 and 20")
		return 2
	}

	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		fs.Usage()
		return 2
	}
	if _, err := exec.LookPath("gh"); err != nil {
		fmt.Fprintln(os.Stderr, "xfdig: GitHub CLI (gh) is required and was not found in PATH")
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client := gh.NewClient(gh.ExecRunner{})
	result, err := finder.New(client).Find(ctx, query, *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "xfdig:", err)
		return 1
	}

	if *text {
		err = output.Text(os.Stdout, result)
	} else {
		err = output.JSON(os.Stdout, result)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "xfdig:", err)
		return 1
	}
	return 0
}
