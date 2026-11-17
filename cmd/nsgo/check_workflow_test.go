package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// check.yml splits -target all between a Linux job and a Mac one, as Apple's targets need Xcode, so a target added to targets must join one of its lists or no workflow vets it. The Linux job also names the HarmonyOS preview, which all leaves out, so its nsgoharmony backend is vetted too.
func TestCheckWorkflowVetsEveryTargetOnce(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/check.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct{ Run string }
		}
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, job := range workflow.Jobs {
		for _, step := range job.Steps {
			if list, ok := strings.CutPrefix(step.Run, "go tool nsgo check -target "); ok {
				got = append(got, strings.Split(strings.TrimSpace(list), ",")...)
			}
		}
	}
	slices.Sort(got)
	want := append(goAndXcodeTargets(), "harmonyos")
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("check.yml vets %v, want each required target %v once", got, want)
	}
}
