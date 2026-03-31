package git

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"
)

type Info struct {
    Branch   string
    Dirty    bool
    Ahead    int
    Behind   int
    Dir            string // directory passed to Fetch; set even when git is unavailable
    IsWorktreeRoot bool   // true when Dir contains a .bare/ subdirectory
    RepoRoot       string
    Worktree string // non-empty when inside a linked worktree (name of the worktree)
    PR       string
    Loading  bool
}

var (
    aheadRe  = regexp.MustCompile(`ahead (\d+)`)
    behindRe = regexp.MustCompile(`behind (\d+)`)
)

func ParseStatus(raw string) (Info, error) {
    var info Info
    lines := strings.Split(raw, "\n")
    if len(lines) == 0 {
        return info, fmt.Errorf("empty output")
    }

    branchLine := strings.TrimPrefix(lines[0], "## ")
    if strings.HasPrefix(branchLine, "HEAD") {
        info.Branch = "HEAD"
    } else {
        parts := strings.SplitN(branchLine, "...", 2)
        info.Branch = parts[0]
    }

    if m := aheadRe.FindStringSubmatch(branchLine); m != nil {
        info.Ahead, _ = strconv.Atoi(m[1])
    }
    if m := behindRe.FindStringSubmatch(branchLine); m != nil {
        info.Behind, _ = strconv.Atoi(m[1])
    }

    for _, line := range lines[1:] {
        if strings.TrimSpace(line) != "" {
            info.Dirty = true
            break
        }
    }
    return info, nil
}

func Fetch(dir string, timeoutMs int) (Info, error) {
    ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
    defer cancel()

    out, err := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain=v1", "-b").Output()
    if err != nil {
        info := Info{Dir: dir}
        if fi, statErr := os.Stat(filepath.Join(dir, ".bare")); statErr == nil && fi.IsDir() {
            info.IsWorktreeRoot = true
        }
        return info, fmt.Errorf("git status: %w", err)
    }
    info, err := ParseStatus(string(out))
    if err != nil {
        return info, err
    }
    info.Dir = dir

    rootOut, _ := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
    info.RepoRoot = strings.TrimSpace(string(rootOut))

    gitDirOut, _ := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--git-dir").Output()
    gitDir := strings.TrimSpace(string(gitDirOut))
    if idx := strings.Index(gitDir, "/worktrees/"); idx >= 0 {
        info.Worktree = gitDir[idx+len("/worktrees/"):]
    }

    return info, nil
}

func FetchPR(dir, branch string, timeoutMs int) string {
    ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
    defer cancel()

    out, err := exec.CommandContext(ctx, "gh", "pr", "list",
        "--head", branch,
        "--state", "open",
        "--json", "number",
        "--jq", ".[0].number",
    ).Output()
    if err != nil || strings.TrimSpace(string(out)) == "" {
        return ""
    }
    return "#" + strings.TrimSpace(string(out)) + " open"
}

func IsDescendant(child, parent string) bool {
    if child == "" || parent == "" {
        return false
    }
    if !strings.HasSuffix(parent, "/") {
        parent += "/"
    }
    return strings.HasPrefix(child, parent)
}
