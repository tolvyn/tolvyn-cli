package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Item SK, 27 Nov: the SDKs were released with their CHANGELOG headings still "— unreleased". The release workflow's first
// job stops unless the CHANGELOG's top heading is the tag's version with a date ("## 1.3.2 — 2026-11-27", or with
// a v), before anything is built or published. These tests run that step's own script.
const changelogStep = "      - name: The CHANGELOG's top heading is this tag's version, dated\n"

func changelogCheck(t *testing.T) string {
	t.Helper()
	wf, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(wf)
	at := strings.Index(s, changelogStep)
	if at < 0 {
		t.Fatal("the release workflow has no CHANGELOG check")
	}
	if needs := strings.Index(s, "\n    needs:"); needs >= 0 && needs < at {
		t.Fatal("the CHANGELOG check is not in the first job, before anything is built or published")
	}
	lines := strings.Split(s[at+len(changelogStep):], "\n")
	if len(lines) < 3 || lines[0] != "        shell: bash" || lines[1] != "        run: |" {
		t.Fatal("the CHANGELOG check is not a bash run: | block")
	}
	var body []string
	for _, l := range lines[2:] {
		if !strings.HasPrefix(l, "          ") {
			break
		}
		body = append(body, l[10:])
	}
	return strings.Join(body, "\n") + "\n"
}

func TestTheReleaseNeedsTheTagsVersionDatedAtTheTopOfTheChangelog(t *testing.T) {
	script := changelogCheck(t)
	cases := []struct {
		top, tag string
		ok       bool
	}{
		{"## 1.3.2 — 2026-11-27", "v1.3.2", true},
		{"## v1.3.2 — 2026-11-27", "v1.3.2", true},
		{"## 1.3.2 — unreleased", "v1.3.2", false},
		{"## 1.3.1 — 2026-11-27", "v1.3.2", false},
		{"## 1.3.20 — 2026-11-27", "v1.3.2", false},
		{"## 1.3.2 — 2026-11-27", "v1.3.20", false},
		{"## 1.3.2", "v1.3.2", false},
		{"## 1.3.2 — 2026-11-27 (draft)", "v1.3.2", false},
		{"## 1.3.2 — 27 Nov 2026", "v1.3.2", false},
		{"## 1.3.3 — unreleased\n\n## 1.3.2 — 2026-11-27", "v1.3.2", false},
		{"", "v1.3.2", false},
	}
	for _, c := range cases {
		t.Run(c.tag+" "+c.top, func(t *testing.T) {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("# Changelog\n\n"+c.top+"\n\n- an entry\n"), 0o644) //nolint:errcheck
			cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", "-c", script)
			cmd.Dir = dir
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GITHUB_REF_NAME=" + c.tag}
			out, err := cmd.CombinedOutput()
			first := strings.SplitN(c.top, "\n", 2)[0]
			want := "CHANGELOG: " + first + "\n"
			if !c.ok {
				want = "CHANGELOG.md's top heading must be '## <version> — YYYY-MM-DD' for " + c.tag + "; it is '" + first + "'\n"
			}
			if (err == nil) != c.ok || string(out) != want {
				t.Errorf("err=%v, output %q, want %q", err, out, want)
			}
		})
	}
}
