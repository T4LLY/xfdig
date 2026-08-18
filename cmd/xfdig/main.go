package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/T4LLY/xfdig/internal/finder"
	gh "github.com/T4LLY/xfdig/internal/github"
	"github.com/T4LLY/xfdig/internal/output"
)

const version = "0.3.0"

var (
	errHelp      = errors.New("help requested")
	relativeTime = regexp.MustCompile(`^([1-9][0-9]*)([dmy])$`)
)

type cliConfig struct {
	Language    string
	Query       string
	Since       string
	Until       string
	Limit       int
	Text        bool
	ShowVersion bool
}

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := parseCLI(os.Args[1:], time.Now())
	if errors.Is(err, errHelp) {
		printUsage(os.Stdout)
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "xfdig:", err)
		printUsage(os.Stderr)
		return 2
	}
	if cfg.ShowVersion {
		fmt.Println("xfdig " + version)
		return 0
	}
	if _, err := exec.LookPath("gh"); err != nil {
		fmt.Fprintln(os.Stderr, "xfdig: GitHub CLI (gh) is required and was not found in PATH")
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client := gh.NewClient(gh.ExecRunner{})
	result, findErr := finder.New(client).Find(ctx, cfg.Query, finder.Options{
		Language: cfg.Language,
		Since:    cfg.Since,
		Until:    cfg.Until,
		Limit:    cfg.Limit,
	})
	if err := writeResult(cfg, result); err != nil {
		fmt.Fprintln(os.Stderr, "xfdig:", err)
		return 1
	}
	if findErr != nil {
		fmt.Fprintln(os.Stderr, "xfdig:", findErr)
		return 1
	}
	return 0
}

func writeResult(cfg cliConfig, result finder.Result) error {
	if cfg.Text {
		return output.Text(os.Stdout, result)
	}
	return output.JSON(os.Stdout, result)
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, `Usage: xfdig <language|any> [options] <bug, error, or symptom>

Find merged GitHub PRs linked to similar closed issues.

Options:
  -s, --since <time>  search issues closed since this time
  -u, --until <time>  search issues closed until this time
  -n <N>              maximum number of fixes (1-100, default 20)
  -t                  human-readable output
      --version        print version
  -h, --help           show help

Time values accept Nd, Nm, Ny, or YYYY-MM-DD. Examples: 14d, 6m, 2y.`)
}

func parseCLI(args []string, now time.Time) (cliConfig, error) {
	cfg := cliConfig{Limit: 20}
	positionals := make([]string, 0, len(args))
	endOptions := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOptions {
			positionals = append(positionals, arg)
			continue
		}

		switch {
		case arg == "--":
			endOptions = true
		case arg == "-h" || arg == "--help":
			return cliConfig{}, errHelp
		case arg == "--version":
			cfg.ShowVersion = true
		case arg == "-t":
			cfg.Text = true
		case arg == "-s" || arg == "--since":
			value, next, err := optionValue(args, i, arg)
			if err != nil {
				return cliConfig{}, err
			}
			i = next
			cfg.Since, err = resolveTime(value, now)
			if err != nil {
				return cliConfig{}, fmt.Errorf("%s: %w", arg, err)
			}
		case strings.HasPrefix(arg, "-s=") || strings.HasPrefix(arg, "--since="):
			option, value, _ := strings.Cut(arg, "=")
			var err error
			cfg.Since, err = resolveTime(value, now)
			if err != nil {
				return cliConfig{}, fmt.Errorf("%s: %w", option, err)
			}
		case arg == "-u" || arg == "--until":
			value, next, err := optionValue(args, i, arg)
			if err != nil {
				return cliConfig{}, err
			}
			i = next
			cfg.Until, err = resolveTime(value, now)
			if err != nil {
				return cliConfig{}, fmt.Errorf("%s: %w", arg, err)
			}
		case strings.HasPrefix(arg, "-u=") || strings.HasPrefix(arg, "--until="):
			option, value, _ := strings.Cut(arg, "=")
			var err error
			cfg.Until, err = resolveTime(value, now)
			if err != nil {
				return cliConfig{}, fmt.Errorf("%s: %w", option, err)
			}
		case arg == "-n":
			value, next, err := optionValue(args, i, arg)
			if err != nil {
				return cliConfig{}, err
			}
			i = next
			cfg.Limit, err = parseLimit(value)
			if err != nil {
				return cliConfig{}, err
			}
		case strings.HasPrefix(arg, "-n="):
			var err error
			cfg.Limit, err = parseLimit(strings.TrimPrefix(arg, "-n="))
			if err != nil {
				return cliConfig{}, err
			}
		case strings.HasPrefix(arg, "-"):
			return cliConfig{}, fmt.Errorf("unknown option %q", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	if cfg.ShowVersion {
		return cfg, nil
	}
	if len(positionals) < 2 {
		return cliConfig{}, fmt.Errorf("language and query are required")
	}

	cfg.Language = strings.TrimSpace(positionals[0])
	cfg.Query = strings.TrimSpace(strings.Join(positionals[1:], " "))
	if cfg.Language == "" {
		return cliConfig{}, fmt.Errorf("language is empty")
	}
	if cfg.Query == "" {
		return cliConfig{}, fmt.Errorf("query is empty")
	}
	if cfg.Since != "" && cfg.Until != "" && cfg.Since > cfg.Until {
		return cliConfig{}, fmt.Errorf("--since must not be later than --until")
	}

	return cfg, nil
}

func optionValue(args []string, index int, option string) (string, int, error) {
	if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
		return "", index, fmt.Errorf("%s requires a value", option)
	}
	return args[index+1], index + 1, nil
}

func parseLimit(raw string) (int, error) {
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > 100 {
		return 0, fmt.Errorf("-n must be between 1 and 100")
	}
	return limit, nil
}

func resolveTime(raw string, now time.Time) (string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "", fmt.Errorf("time is empty")
	}

	if absolute, err := time.ParseInLocation("2006-01-02", raw, now.Location()); err == nil {
		return absolute.Format("2006-01-02"), nil
	}

	match := relativeTime.FindStringSubmatch(raw)
	if match == nil {
		return "", fmt.Errorf("invalid time %q; use Nd, Nm, Ny, or YYYY-MM-DD", raw)
	}

	amount, err := strconv.Atoi(match[1])
	if err != nil {
		return "", fmt.Errorf("invalid time %q", raw)
	}

	var resolved time.Time
	switch match[2] {
	case "d":
		resolved = now.AddDate(0, 0, -amount)
	case "m":
		resolved = subtractCalendarMonths(now, amount)
	case "y":
		resolved = subtractCalendarMonths(now, amount*12)
	default:
		return "", fmt.Errorf("invalid time %q", raw)
	}
	return resolved.Format("2006-01-02"), nil
}

func subtractCalendarMonths(t time.Time, months int) time.Time {
	totalMonths := t.Year()*12 + int(t.Month()) - 1 - months
	year := totalMonths / 12
	month := time.Month(totalMonths%12 + 1)
	day := t.Day()
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, t.Location()).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}
