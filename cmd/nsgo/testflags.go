package main

import (
	"flag"
	"fmt"
	"io"
	"iter"
	"strconv"
	"strings"
)

type targetTestFlags struct {
	args     []string
	count    int
	cpus     []string
	failfast bool
}

// executions keeps Go's in-process repetitions for libraries. A linked native
// host runs each repetition and CPU setting in a fresh process: phone and web
// hosts can open only one app, and desktop tests get the same isolation.
func (f targetTestFlags) executions(hosted bool) iter.Seq[[]string] {
	base, cpus := []string{}, f.cpus
	if len(cpus) == 0 {
		cpus = []string{""}
	}
	for _, arg := range f.args {
		switch {
		case strings.HasPrefix(arg, "-test.count="):
		case strings.HasPrefix(arg, "-test.cpu="):
		default:
			base = append(base, arg)
		}
	}
	return func(yield func([]string) bool) {
		if !hosted {
			yield(f.args)
			return
		}
		for _, cpu := range cpus {
			for range f.count {
				args := append([]string{}, base...)
				args = append(args, "-test.count=1")
				if cpu != "" {
					args = append(args, "-test.cpu="+cpu)
				}
				if !yield(args) {
					return
				}
			}
		}
	}
}

// parseTargetTestFlags accepts only flags the target runner can honor. Build,
// profiling and discovery flags need go test, and the runner itself owns run,
// timeout and verbose output; none may disappear on a target.
func parseTargetTestFlags(args []string) (targetTestFlags, error) {
	var f targetTestFlags
	fs := flag.NewFlagSet("target test flags", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Bool("short", false, "")
	fs.Bool("fullpath", false, "")
	fs.BoolVar(&f.failfast, "failfast", false, "")
	fs.IntVar(&f.count, "count", 1, "")
	parallel := fs.Int("parallel", 1, "")
	cpu := fs.String("cpu", "", "")
	normal := make([]string, len(args))
	for i, arg := range args {
		if name, ok := strings.CutPrefix(arg, "--test."); ok {
			arg = "-" + name
		} else if name, ok := strings.CutPrefix(arg, "-test."); ok {
			arg = "-" + name
		}
		normal[i] = arg
	}
	if err := fs.Parse(normal); err != nil {
		return f, fmt.Errorf("test: target runs cannot accept these go test arguments: %w; put NSGo's -run and -timeout before the directory, or run headless for other go test flags", err)
	}
	if fs.NArg() != 0 {
		return f, fmt.Errorf("test: unexpected target test argument %q; pass only supported go test flags after --", fs.Arg(0))
	}
	if *parallel <= 0 {
		return f, fmt.Errorf("test: -parallel must be positive")
	}
	if *cpu != "" {
		f.cpus = strings.Split(*cpu, ",")
		for _, value := range f.cpus {
			if n, err := strconv.Atoi(value); err != nil || n <= 0 {
				return f, fmt.Errorf("test: -cpu wants comma-separated positive integers, got %q", *cpu)
			}
		}
	}
	fs.Visit(func(option *flag.Flag) {
		f.args = append(f.args, "-test."+option.Name+"="+option.Value.String())
	})
	return f, nil
}
