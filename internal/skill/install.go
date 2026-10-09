// Package skill installs the portable Pitstop agent skill bundle.
package skill

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"pitstop/internal/agentassets"
	"pitstop/internal/config"
	"pitstop/internal/model"
)

type Options struct {
	Agent  string
	Scope  string
	Repo   string
	DryRun bool
	Force  bool
}

const (
	bundleDirName   = "pitstop-config"
	lockName        = ".pitstop-config.lock"
	stagePrefix     = ".pitstop-config.stage-"
	backupPrefix    = ".pitstop-config.backup-"
	codexSkillsPath = ".agents/skills/" + bundleDirName
	claudePath      = ".claude/skills/" + bundleDirName
)

type manifest struct {
	Owner         string          `json:"owner"`
	BundleVersion string          `json:"bundleVersion"`
	Files         []manifestEntry `json:"files"`
}

type manifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type bundle struct {
	files  []string
	data   map[string][]byte
	hashes map[string]string
}

// These narrow hooks keep transaction boundary failures testable without introducing a filesystem abstraction.
var installRename = func(root *os.Root, oldPath, newPath string) error { return root.Rename(oldPath, newPath) }
var installWrite = writeAll
var installAfterPublish func()
var installAfterBackupMove func()

func Install(ctx context.Context, opts Options) (result model.SkillInstallResult, retErr error) {
	result = model.SkillInstallResult{
		Agent:    opts.Agent,
		Scope:    opts.Scope,
		DryRun:   opts.DryRun,
		Action:   "install",
		Warnings: []string{},
	}
	if opts.Agent != "codex" && opts.Agent != "claude" {
		return result, installError("SKILL_INSTALL_FAILED", "unsupported agent", nil)
	}
	if opts.Scope != "user" && opts.Scope != "project" {
		return result, installError("SKILL_INSTALL_FAILED", "unsupported scope", nil)
	}
	if opts.Scope == "user" && opts.Repo != "" {
		return result, installError("SKILL_INSTALL_FAILED", "--repo is only valid for project scope", nil)
	}
	if err := contextError(ctx); err != nil {
		return result, err
	}
	b, err := loadBundle()
	if err != nil {
		return result, err
	}
	result.Files = append([]string(nil), b.files...)
	result.BundleVersion = agentassets.BundleVersion

	rootPath, targetRel, err := destination(opts)
	if cancelErr := contextError(ctx); cancelErr != nil {
		return result, cancelErr
	}
	if err != nil {
		return result, err
	}
	rootPath, err = filepath.Abs(rootPath)
	if err != nil {
		return result, installError("SKILL_INSTALL_FAILED", "could not resolve installation root", err)
	}
	result.TargetPath = filepath.Join(rootPath, filepath.FromSlash(targetRel))
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return result, installError("SKILL_INSTALL_FAILED", "could not open installation root", err)
	}
	defer root.Close()

	if err := ensureAncestors(root, targetRel); err != nil {
		result.Action = "blocked"
		return result, installError("SKILL_INSTALL_FAILED", "unsafe skill destination", err)
	}
	exists, err := targetState(root, targetRel)
	if err != nil {
		result.Action = "blocked"
		return result, installError("SKILL_INSTALL_FAILED", "unsafe skill destination", err)
	}
	parentRel := filepath.Dir(targetRel)
	if recovery, scanErr := retainedBackups(root, parentRel); scanErr != nil {
		result.Action = "blocked"
		return result, installError("SKILL_INSTALL_FAILED", "could not inspect skill recovery state", scanErr)
	} else if recovery != "" {
		result.Action = "blocked"
		return result, installError("SKILL_INSTALL_FAILED", "retained backup requires recovery: "+recovery, nil)
	}
	if exists {
		if opts.Force {
			result.Action = "replace"
		}
		if !opts.Force {
			result.Action = "blocked"
			if opts.DryRun {
				return result, nil
			}
			return result, installError("SKILL_EXISTS", "Pitstop skill is already installed", nil)
		}
		if err := verifyOwnedBundle(root, targetRel); err != nil {
			result.Action = "blocked"
			return result, installError("SKILL_INSTALL_FAILED", "existing skill is not an owned complete bundle", err)
		}
	}
	if opts.DryRun {
		return result, nil
	}

	if err := mkdirAll(root, parentRel, 0700); err != nil {
		return result, installError("SKILL_INSTALL_FAILED", "could not create skill parent", err)
	}
	if err := ensureAncestors(root, targetRel); err != nil {
		return result, installError("SKILL_INSTALL_FAILED", "unsafe skill destination", err)
	}
	if recovery, scanErr := retainedBackups(root, parentRel); scanErr != nil {
		return result, installError("SKILL_INSTALL_FAILED", "could not inspect skill recovery state", scanErr)
	} else if recovery != "" {
		return result, installError("SKILL_INSTALL_FAILED", "retained backup requires recovery: "+recovery, nil)
	}

	lockRel := filepath.Join(parentRel, lockName)
	lock, err := root.OpenFile(lockRel, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return result, installError("SKILL_INSTALL_FAILED", "could not acquire skill install lock", err)
	}
	if err := lock.Close(); err != nil {
		if cleanupErr := root.Remove(lockRel); cleanupErr != nil {
			result.Warnings = append(result.Warnings, "could not remove installer lock; recovery path: "+filepath.Join(rootPath, filepath.FromSlash(lockRel)))
		}
		return result, installError("SKILL_INSTALL_FAILED", "could not close skill install lock", err)
	}
	defer func() {
		if cleanupErr := root.Remove(lockRel); cleanupErr != nil {
			result.Warnings = append(result.Warnings, "could not remove installer lock; recovery path: "+filepath.Join(rootPath, filepath.FromSlash(lockRel)))
		}
	}()

	// The target is checked again after taking the sibling lock.
	exists, err = targetState(root, targetRel)
	if err != nil {
		return result, installError("SKILL_INSTALL_FAILED", "unsafe skill destination", err)
	}
	if exists {
		if opts.Force {
			result.Action = "replace"
		}
		if !opts.Force {
			result.Action = "blocked"
			return result, installError("SKILL_EXISTS", "Pitstop skill is already installed", nil)
		}
		if err := verifyOwnedBundle(root, targetRel); err != nil {
			result.Action = "blocked"
			return result, installError("SKILL_INSTALL_FAILED", "existing skill is not an owned complete bundle", err)
		}
	}
	if err := contextError(ctx); err != nil {
		return result, err
	}

	suffix, err := uniqueSuffix()
	if err != nil {
		return result, installError("SKILL_INSTALL_FAILED", "could not create unique staging name", err)
	}
	stageRel := filepath.Join(parentRel, stagePrefix+suffix)
	if err := root.Mkdir(stageRel, 0700); err != nil {
		return result, installError("SKILL_INSTALL_FAILED", "could not create staging directory", err)
	}
	if err := writeBundle(root, stageRel, b); err != nil {
		cleanupOwned(root, stageRel, rootPath, &result)
		return result, installError("SKILL_INSTALL_FAILED", "could not stage skill bundle", err)
	}
	if err := verifyBundle(root, stageRel, b); err != nil {
		cleanupOwned(root, stageRel, rootPath, &result)
		return result, installError("SKILL_INSTALL_FAILED", "staged skill bundle failed verification", err)
	}
	if err := contextError(ctx); err != nil {
		cleanupOwned(root, stageRel, rootPath, &result)
		return result, err
	}

	backupRel := ""
	if exists {
		// Backup and publish are separate renames; a crash between them can leave a retained backup for recovery.
		backupRel = filepath.Join(parentRel, backupPrefix+suffix)
		if err := installRename(root, targetRel, backupRel); err != nil {
			cleanupOwned(root, stageRel, rootPath, &result)
			return result, installError("SKILL_INSTALL_FAILED", "could not retain existing skill bundle", err)
		}
		if installAfterBackupMove != nil {
			installAfterBackupMove()
		}
		if err := contextError(ctx); err != nil {
			if rollbackErr := installRename(root, backupRel, targetRel); rollbackErr != nil {
				cleanupOwned(root, stageRel, rootPath, &result)
				return rollbackFailure(result, root.Name(), backupRel, rollbackErr)
			}
			cleanupOwned(root, stageRel, rootPath, &result)
			return result, err
		}
	}
	if err := installRename(root, stageRel, targetRel); err != nil {
		cleanupOwned(root, stageRel, rootPath, &result)
		if backupRel != "" {
			if rollbackErr := installRename(root, backupRel, targetRel); rollbackErr != nil {
				cleanupOwned(root, stageRel, rootPath, &result)
				return rollbackFailure(result, root.Name(), backupRel, rollbackErr)
			}
		}
		return result, installError("SKILL_INSTALL_FAILED", "could not publish skill bundle", err)
	}
	if installAfterPublish != nil {
		installAfterPublish()
	}
	result.Installed = true

	if backupRel != "" {
		if err := root.RemoveAll(backupRel); err != nil {
			result.Warnings = append(result.Warnings, "could not remove backup; recovery path: "+filepath.Join(rootPath, filepath.FromSlash(backupRel)))
		}
	}
	if ctx.Err() != nil {
		result.Warnings = append(result.Warnings, "cancellation observed after publish; installation completed")
	}
	return result, nil
}

func destination(opts Options) (string, string, error) {
	if opts.Scope == "user" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", installError("SKILL_INSTALL_FAILED", "could not resolve user home", err)
		}
		if opts.Agent == "codex" {
			return home, codexSkillsPath, nil
		}
		return home, claudePath, nil
	}
	repo, err := config.ResolveRepo(opts.Repo)
	if err != nil {
		return "", "", installError("SKILL_INSTALL_FAILED", "could not resolve project repository", err)
	}
	if opts.Agent == "codex" {
		return repo, codexSkillsPath, nil
	}
	return repo, claudePath, nil
}

func loadBundle() (bundle, error) {
	files := agentassets.Files()
	sort.Strings(files)
	data := make(map[string][]byte, len(files))
	for _, path := range files {
		if path == "" || !isValidBundlePath(path) {
			return bundle{}, installError("SKILL_INSTALL_FAILED", "invalid embedded bundle path", nil)
		}
		contents, err := agentassets.ReadFile(path)
		if err != nil {
			return bundle{}, installError("SKILL_INSTALL_FAILED", "embedded bundle is incomplete", err)
		}
		data[path] = contents
	}
	m, err := parseManifest(data["manifest.json"])
	if err != nil || m.BundleVersion != agentassets.BundleVersion {
		return bundle{}, installError("SKILL_INSTALL_FAILED", "embedded bundle manifest is invalid", err)
	}
	want := append([]string(nil), files...)
	want = removeString(want, "manifest.json")
	got := make([]string, 0, len(m.Files))
	hashes := map[string]string{"manifest.json": hashBytes(data["manifest.json"])}
	for _, entry := range m.Files {
		got = append(got, entry.Path)
		contents, ok := data[entry.Path]
		if !ok || entry.Path == "manifest.json" {
			return bundle{}, installError("SKILL_INSTALL_FAILED", "embedded bundle manifest lists an invalid file", nil)
		}
		hash := sha256.Sum256(contents)
		if entry.SHA256 != hex.EncodeToString(hash[:]) {
			return bundle{}, installError("SKILL_INSTALL_FAILED", "embedded bundle manifest hash mismatch", nil)
		}
		hashes[entry.Path] = entry.SHA256
	}
	if !reflectStringsEqual(got, want) {
		return bundle{}, installError("SKILL_INSTALL_FAILED", "embedded bundle manifest is incomplete", nil)
	}
	return bundle{files: files, data: data, hashes: hashes}, nil
}

func verifyOwnedBundle(root *os.Root, target string) error {
	manifestPath := filepath.ToSlash(filepath.Join(target, "manifest.json"))
	trustedManifests := agentassets.TrustedBundles()
	maxManifestSize := 0
	for _, trusted := range trustedManifests {
		if len(trusted) > maxManifestSize {
			maxManifestSize = len(trusted)
		}
	}
	manifestBytes, err := readRegular(root, manifestPath, maxManifestSize)
	if err != nil {
		return err
	}
	for _, trusted := range trustedManifests {
		if !bytes.Equal(manifestBytes, trusted) {
			continue
		}
		m, err := parseManifest(trusted)
		if err != nil {
			return errors.New("trusted bundle manifest is invalid")
		}
		files := make([]string, 0, len(m.Files)+1)
		hashes := map[string]string{"manifest.json": hashBytes(trusted)}
		for _, entry := range m.Files {
			files = append(files, entry.Path)
			hashes[entry.Path] = entry.SHA256
		}
		files = append(files, "manifest.json")
		sort.Strings(files)
		return verifyBundle(root, target, bundle{files: files, hashes: hashes})
	}
	return errors.New("destination manifest is not a trusted Pitstop manifest")
}

func writeBundle(root *os.Root, stage string, b bundle) error {
	for _, path := range b.files {
		rel := filepath.Join(stage, filepath.FromSlash(path))
		if err := mkdirAll(root, filepath.Dir(rel), 0700); err != nil {
			return err
		}
		f, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
		if err != nil {
			return err
		}
		if err := installWrite(f, b.data[path]); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

func verifyBundle(root *os.Root, target string, b bundle) error {
	info, err := root.Lstat(target)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("bundle destination is not a directory")
	}
	expected := make(map[string]bool, len(b.files))
	dirs := map[string]bool{".": true}
	for _, path := range b.files {
		expected[path] = true
		for dir := filepath.Dir(filepath.FromSlash(path)); dir != "."; dir = filepath.Dir(dir) {
			dirs[dir] = true
		}
	}
	seen := map[string]bool{}
	targetPath := filepath.ToSlash(target)
	if err := fs.WalkDir(root.FS(), targetPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == targetPath {
			return nil
		}
		rel := strings.TrimPrefix(path, targetPath+"/")
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in bundle: %s", rel)
		}
		if entry.IsDir() {
			if !dirs[filepath.FromSlash(rel)] {
				return fmt.Errorf("unknown bundle directory: %s", rel)
			}
			return nil
		}
		if !expected[rel] {
			return fmt.Errorf("unknown bundle file: %s", rel)
		}
		info, err := entry.Info()
		data, hasData := b.data[rel]
		maxSize := int64(1 << 20)
		if hasData {
			maxSize = int64(len(data))
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxSize || (hasData && info.Size() != int64(len(data))) {
			return fmt.Errorf("invalid bundle file: %s", rel)
		}
		contents, err := readRegular(root, filepath.ToSlash(filepath.Join(target, filepath.FromSlash(rel))), int(maxSize))
		if err != nil || (hasData && !bytes.Equal(contents, data)) || hashBytes(contents) != b.hashes[rel] {
			return fmt.Errorf("bundle content mismatch: %s", rel)
		}
		seen[rel] = true
		return nil
	}); err != nil {
		return err
	}
	for _, path := range b.files {
		if !seen[path] {
			return fmt.Errorf("missing bundle file: %s", path)
		}
	}
	return nil
}

func readRegular(root *os.Root, path string, size int) ([]byte, error) {
	f, err := root.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > int64(size) {
		return nil, errors.New("not a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(size)+1))
	if err != nil || len(data) > size {
		return nil, errors.New("file exceeds expected size")
	}
	return data, nil
}

func parseManifest(data []byte) (manifest, error) {
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil || m.Owner != "pitstop" || len(m.Files) == 0 {
		return manifest{}, errors.New("invalid manifest")
	}
	for _, entry := range m.Files {
		if !isValidBundlePath(entry.Path) || len(entry.SHA256) != sha256.Size*2 {
			return manifest{}, errors.New("invalid manifest file entry")
		}
	}
	return m, nil
}

func hashBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func targetState(root *os.Root, target string) (bool, error) {
	info, err := root.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, errors.New("destination is a symlink or non-directory")
	}
	return true, nil
}

func ensureAncestors(root *os.Root, target string) error {
	clean := filepath.Clean(target)
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("destination escapes installation root")
	}
	parts := strings.Split(filepath.ToSlash(clean), "/")
	current := ""
	for _, part := range parts[:len(parts)-1] {
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}
		info, err := root.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("destination ancestor is not a directory")
		}
	}
	return nil
}

func mkdirAll(root *os.Root, path string, mode os.FileMode) error {
	path = filepath.Clean(path)
	if path == "." || path == "" {
		return nil
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	current := ""
	for _, part := range parts {
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}
		if err := root.Mkdir(current, mode); err != nil {
			info, statErr := root.Lstat(current)
			if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return err
			}
		}
	}
	return nil
}

func retainedBackups(root *os.Root, parent string) (string, error) {
	dir, err := root.Open(parent)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), backupPrefix) {
			return filepath.Join(root.Name(), filepath.FromSlash(parent), entry.Name()), nil
		}
	}
	return "", nil
}

func uniqueSuffix() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]) + fmt.Sprintf("-%x", time.Now().UnixNano()), nil
}

func rollbackFailure(result model.SkillInstallResult, root, backup string, err error) (model.SkillInstallResult, error) {
	result.Warnings = append(result.Warnings, "retained backup for recovery: "+filepath.Join(root, filepath.FromSlash(backup)))
	return result, installError("SKILL_INSTALL_FAILED", "publish failed and rollback failed; recover from retained backup", err)
}

func cleanupOwned(root *os.Root, rel, rootPath string, result *model.SkillInstallResult) {
	if err := root.RemoveAll(rel); err != nil {
		result.Warnings = append(result.Warnings, "could not clean temporary installer state; recovery path: "+filepath.Join(rootPath, filepath.FromSlash(rel)))
	}
}

func contextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return installError("CANCELLED", "skill installation cancelled", err)
	}
	return nil
}

func installError(code, message string, cause error) error { return model.Err(code, message, cause) }

func isValidBundlePath(path string) bool {
	return fs.ValidPath(path) && filepath.ToSlash(path) == path
}

func removeString(values []string, target string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func reflectStringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func writeAll(f *os.File, data []byte) error {
	for len(data) > 0 {
		n, err := f.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
