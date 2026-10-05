package snapshot

import (
	"context"
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
	return CaptureContext(context.Background(), repo, runDir)
}
func CaptureContext(ctx context.Context, repo, runDir string) (model.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return model.Snapshot{}, model.Err("CANCELLED", "snapshot capture cancelled", err)
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", repo, "diff", "--name-only", "--diff-filter=U").Output(); err != nil {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "cannot inspect merge state", err)
	} else if len(out) > 0 {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "unresolved merge state", nil)
	}
	headBytes, err := exec.CommandContext(ctx, "git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "repository has no HEAD", err)
	}
	root, absErr := filepath.Abs(repo)
	if absErr != nil {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "cannot resolve repository path", absErr)
	}
	indexBefore, ie := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--stage", "-z").Output()
	if ie != nil {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "cannot inspect index", ie)
	}
	if sub, e := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--stage").Output(); e != nil {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "cannot inspect submodules", e)
	} else if hasSubmodule(string(sub)) {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "submodules are unsupported", nil)
	}
	tracked, err := trackedFiles(ctx, root)
	if err != nil {
		return model.Snapshot{}, err
	}
	allowed, err := sourceFiles(ctx, root)
	if err != nil {
		return model.Snapshot{}, err
	}
	statusBefore, statusErr := gitStatus(ctx, root)
	if statusErr != nil {
		return model.Snapshot{}, statusErr
	}
	before, err := manifest(ctx, root, tracked, allowed)
	if err != nil {
		return model.Snapshot{}, err
	}
	clone := filepath.Join(runDir, "snapshot")
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(clone)
		}
	}()
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return model.Snapshot{}, err
	}
	if out, err := exec.CommandContext(ctx, "git", "clone", "--no-hardlinks", "--local", root, clone).CombinedOutput(); err != nil {
		_ = os.RemoveAll(clone)
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "cannot clone source", fmt.Errorf("%v: %s", err, out))
	}
	entries, e := os.ReadDir(clone)
	if e != nil {
		return model.Snapshot{}, e
	}
	for _, ent := range entries {
		if ent.Name() != ".git" {
			if e := os.RemoveAll(filepath.Join(clone, ent.Name())); e != nil {
				return model.Snapshot{}, e
			}
		}
	}
	excluded := []string{}
	if err := overlay(ctx, root, clone, "", &excluded, tracked, allowed, root); err != nil {
		return model.Snapshot{}, err
	}
	after, err := manifest(ctx, root, tracked, allowed)
	if err != nil {
		return model.Snapshot{}, err
	}
	trackedAfter, te := trackedFiles(ctx, root)
	if te != nil {
		return model.Snapshot{}, te
	}
	allowedAfter, ae := sourceFiles(ctx, root)
	if ae != nil {
		return model.Snapshot{}, ae
	}
	statusAfter, se := gitStatus(ctx, root)
	if se != nil {
		return model.Snapshot{}, se
	}
	headAfter, he := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD").Output()
	if he != nil {
		return model.Snapshot{}, he
	}
	indexAfter, ie := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--stage", "-z").Output()
	if ie != nil {
		return model.Snapshot{}, ie
	}
	cloneHead, ce := exec.CommandContext(ctx, "git", "-C", clone, "rev-parse", "HEAD").Output()
	if ce != nil {
		return model.Snapshot{}, ce
	}
	if before != after || string(headBytes) != string(headAfter) || string(indexBefore) != string(indexAfter) || string(headBytes) != string(cloneHead) || !sameSet(tracked, trackedAfter) || !sameSet(allowed, allowedAfter) || statusBefore != statusAfter {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "working tree changed while capturing", nil)
	}
	captured, err := manifest(ctx, clone, tracked, allowed)
	if err != nil || captured != after {
		return model.Snapshot{}, model.Err("SNAPSHOT_CHANGED", "captured source differs from working tree", err)
	}
	path := filepath.Join(runDir, "manifest.txt")
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		return model.Snapshot{}, err
	}
	sum := sha256.Sum256([]byte(before))
	dirty := strings.TrimSpace(statusAfter) != ""
	success = true
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
func gitStatus(ctx context.Context, root string) (string, error) {
	b, e := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain=v1", "-z").Output()
	return string(b), e
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
func trackedFiles(ctx context.Context, root string) (map[string]bool, error) {
	out := map[string]bool{}
	b := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z")
	raw, err := b.Output()
	if err != nil {
		return nil, err
	}
	for _, p := range strings.Split(string(raw), "\x00") {
		if p != "" {
			out[filepath.Clean(p)] = true
		}
	}
	return out, nil
}
func sourceFiles(ctx context.Context, root string) (map[string]bool, error) {
	out, err := trackedFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	raw, e := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-o", "--exclude-standard", "-z").Output()
	if e != nil {
		return nil, e
	}
	for _, p := range strings.Split(string(raw), "\x00") {
		if p != "" {
			out[filepath.Clean(p)] = true
		}
	}
	return out, nil
}
func overlay(ctx context.Context, src, dst, rel string, excluded *[]string, tracked, allowed map[string]bool, root string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
			if err := overlay(ctx, src, dst, r, excluded, tracked, allowed, root); err != nil {
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
		if !info.Mode().IsRegular() {
			return model.Err("SNAPSHOT_CHANGED", "unsupported special file: "+r, nil)
		}
		if err := os.MkdirAll(filepath.Dir(dp), 0700); err != nil {
			return err
		}
		in, err := os.Open(sp)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := copyContext(ctx, out, in)
		in.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err := os.Chmod(dp, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}
func copyContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 64*1024)
	var n int64
	for {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			n += int64(nw)
			if ew != nil {
				return n, ew
			}
			if nw != nr {
				return n, io.ErrShortWrite
			}
		}
		if er == io.EOF {
			return n, nil
		}
		if er != nil {
			return n, er
		}
	}
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
func manifest(ctx context.Context, root string, tracked, allowed map[string]bool) (string, error) {
	var paths []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, e error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
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
