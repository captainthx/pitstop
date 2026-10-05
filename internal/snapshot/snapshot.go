package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"pitstop/internal/model"
	"sort"
	"strings"
)

func Capture(repo, runDir string) (model.Snapshot, error) {
	if out, err := exec.Command("git", "-C", repo, "diff", "--name-only", "--diff-filter=U").Output(); err == nil && len(out) > 0 {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "unresolved merge state", nil)
	}
	headBytes, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "repository has no HEAD", err)
	}
	root, _ := filepath.Abs(repo)
	if sub, _ := exec.Command("git", "-C", root, "ls-files", "--stage").Output(); hasSubmodule(string(sub)) {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "submodules are unsupported", nil)
	}
	tracked := trackedFiles(root)
	allowed := sourceFiles(root)
	statusBefore := gitStatus(root)
	clone := filepath.Join(runDir, "snapshot")
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return model.Snapshot{}, err
	}
	if out, err := exec.Command("git", "clone", "--no-hardlinks", "--local", root, clone).CombinedOutput(); err != nil {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "cannot clone source", fmt.Errorf("%v: %s", err, out))
	}
	before, err := manifest(root, tracked, allowed)
	if err != nil {
		return model.Snapshot{}, err
	}
	for _, ent := range []string{".git"} {
		_ = ent
	}
	if entries, e := os.ReadDir(clone); e == nil {
		for _, ent := range entries {
			if ent.Name() != ".git" {
				_ = os.RemoveAll(filepath.Join(clone, ent.Name()))
			}
		}
	}
	excluded := []string{}
	if err := overlay(root, clone, "", &excluded, tracked, allowed, root); err != nil {
		return model.Snapshot{}, err
	}
	after, err := manifest(root, tracked, allowed)
	if err != nil {
		return model.Snapshot{}, err
	}
	trackedAfter, allowedAfter := trackedFiles(root), sourceFiles(root)
	if before != after || !sameSet(tracked, trackedAfter) || !sameSet(allowed, allowedAfter) || statusBefore != gitStatus(root) {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "working tree changed while capturing", nil)
	}
	captured, err := manifest(clone, tracked, allowed)
	if err != nil || captured != after {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "captured source differs from working tree", err)
	}
	path := filepath.Join(runDir, "manifest.txt")
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		return model.Snapshot{}, err
	}
	sum := sha256.Sum256([]byte(before))
	dirty := strings.TrimSpace(string(mustOutput("git", "-C", root, "status", "--porcelain"))) != ""
	return model.Snapshot{Path: clone, ManifestPath: path, SourceManifestPath: path, Head: strings.TrimSpace(string(headBytes)), SnapshotSHA256: hex.EncodeToString(sum[:]), Dirty: dirty, ExcludedPaths: excluded, CheckoutMode: "local-source-copy"}, nil
}
func hasSubmodule(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "160000 ") {
			return true
		}
	}
	return false
}
func gitStatus(root string) string {
	b, _ := exec.Command("git", "-C", root, "status", "--porcelain=v1", "-z").Output()
	return string(b)
}
func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
func trackedFiles(root string) map[string]bool {
	out := map[string]bool{}
	b, _ := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	for _, p := range strings.Split(string(b), "\x00") {
		if p != "" {
			out[filepath.Clean(p)] = true
		}
	}
	return out
}
func sourceFiles(root string) map[string]bool {
	out := trackedFiles(root)
	b, _ := exec.Command("git", "-C", root, "ls-files", "-o", "--exclude-standard", "-z").Output()
	for _, p := range strings.Split(string(b), "\x00") {
		if p != "" {
			out[filepath.Clean(p)] = true
		}
	}
	return out
}
func overlay(src, dst, rel string, excluded *[]string, tracked, allowed map[string]bool, root string) error {
	ents, err := os.ReadDir(filepath.Join(src, rel))
	if err != nil {
		return err
	}
	for _, e := range ents {
		name := e.Name()
		if rel == "" && name == ".git" {
			continue
		}
		r := filepath.Join(rel, name)
		if conventional(r) && !tracked[filepath.Clean(r)] {
			*excluded = append(*excluded, r)
			continue
		}
		sp := filepath.Join(src, r)
		dp := filepath.Join(dst, r)
		info, err := os.Lstat(sp)
		if err != nil {
			return err
		}
		if !tracked[filepath.Clean(r)] && !allowed[filepath.Clean(r)] && !hasAllowedPrefix(allowed, filepath.Clean(r)+string(filepath.Separator)) {
			continue
		}
		if info.IsDir() {
			if err := os.MkdirAll(dp, info.Mode().Perm()); err != nil {
				return err
			}
			if err := overlay(src, dst, r, excluded, tracked, allowed, root); err != nil {
				return err
			}
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(sp)
			if err != nil {
				return err
			}
			if filepath.IsAbs(target) {
				return model.Err("SNAPSHOT_CHANGED", "absolute source symlinks are unsupported in isolated snapshots: "+r, nil)
			}
			real := target
			if !filepath.IsAbs(real) {
				real = filepath.Join(filepath.Dir(sp), target)
			}
			real, _ = filepath.EvalSymlinks(real)
			rootReal, _ := filepath.EvalSymlinks(root)
			relReal, relErr := filepath.Rel(rootReal, real)
			if relErr != nil || relReal == ".." || strings.HasPrefix(relReal, ".."+string(filepath.Separator)) {
				return model.Err("SNAPSHOT_CHANGED", "source symlink escapes repository: "+r, nil)
			}
			if _, err := os.Stat(real); err != nil {
				return model.Err("SNAPSHOT_CHANGED", "broken source symlink: "+r, err)
			}
			_ = os.Remove(dp)
			if err := os.Symlink(target, dp); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dp), 0700); err != nil {
			return err
		}
		b, err := os.ReadFile(sp)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dp, b, info.Mode().Perm()); err != nil {
			return err
		}
		if err := os.Chmod(dp, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}
func conventional(p string) bool {
	base := filepath.Base(p)
	return base == ".actrc" || base == ".env" || strings.HasPrefix(base, ".env.") || base == ".secrets" || base == ".vars" || base == ".input"
}
func hasAllowedPrefix(allowed map[string]bool, prefix string) bool {
	for p := range allowed {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}
func manifest(root string, tracked, allowed map[string]bool) (string, error) {
	var paths []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if conventional(rel) && !tracked[filepath.Clean(rel)] {
			return nil
		}
		if !tracked[filepath.Clean(rel)] && !allowed[filepath.Clean(rel)] {
			if info.IsDir() && hasAllowedPrefix(allowed, filepath.Clean(rel)+string(filepath.Separator)) {
				paths = append(paths, rel)
				return nil
			}
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, rel := range paths {
		p := filepath.Join(root, rel)
		i, err := os.Lstat(p)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "%d:%s:%o:", len(rel), rel, i.Mode())
		if i.Mode()&os.ModeSymlink != 0 {
			t, _ := os.Readlink(p)
			fmt.Fprintf(&b, "L:%d:%s\n", len(t), t)
			continue
		}
		if i.Mode().IsRegular() {
			f, err := os.Open(p)
			if err != nil {
				return "", err
			}
			h := sha256.New()
			n, err := io.Copy(h, f)
			f.Close()
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "F:%d:%x\n", n, h.Sum(nil))
		} else {
			b.WriteString("D\n")
		}
	}
	return b.String(), nil
}
func Remove(s model.Snapshot) error {
	if s.Path != "" {
		return os.RemoveAll(s.Path)
	}
	return nil
}
func mustOutput(args ...string) []byte { b, _ := exec.Command(args[0], args[1:]...).Output(); return b }
