package config

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"pitstop/internal/agentassets"
	"pitstop/internal/model"
)

type replaceOnCancelContext struct {
	context.Context
	path  string
	calls int
}

type cancelAfterOpenContext struct {
	context.Context
	calls int
}

func (c *cancelAfterOpenContext) Err() error {
	c.calls++
	if c.calls == 4 {
		return context.Canceled
	}
	return nil
}

func (c *replaceOnCancelContext) Err() error {
	c.calls++
	if c.calls == 5 {
		old := c.path + ".old"
		_ = os.Rename(c.path, old)
		_ = os.WriteFile(c.path, []byte("replacement"), 0600)
		return context.Canceled
	}
	return nil
}

func initRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	return d
}

func TestInitPreservesExistingFile(t *testing.T) {
	d := initRepo(t)
	want := []byte("keep me")
	path := filepath.Join(d, ".pitstop.json")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Init(context.Background(), d, InitOptions{Preset: "base", Architecture: "linux/amd64"})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != "CONFIG_EXISTS" {
		t.Fatalf("error=%v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != string(want) {
		t.Fatalf("existing file changed: %q, %v", got, readErr)
	}
}

func TestInitDryRunAndOverrides(t *testing.T) {
	d := initRepo(t)
	commands := []string{"printf 'a,b\\n'\n", "echo '$HOME'"}
	result, err := Init(context.Background(), d, InitOptions{
		Preset:       "custom",
		Image:        "alpine:3.22",
		Architecture: "linux/amd64",
		Commands:     commands,
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.DryRun || result.Written || result.DestinationExists || result.Config.Checks[0].Steps[0].Run != commands[0] || result.Config.Checks[0].Steps[1].Run != commands[1] {
		t.Fatalf("unexpected dry-run result: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(d, ".pitstop.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created destination: %v", err)
	}
}

func TestInitRejectsSymlinkDestination(t *testing.T) {
	d := initRepo(t)
	target := filepath.Join(d, "target")
	if err := os.WriteFile(target, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(d, ".pitstop.json")); err != nil {
		t.Fatal(err)
	}
	_, err := Init(context.Background(), d, InitOptions{Preset: "base", Architecture: "linux/amd64"})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != "CONFIG_EXISTS" {
		t.Fatalf("error=%v", err)
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil || string(got) != "outside" {
		t.Fatalf("symlink target changed: %q, %v", got, readErr)
	}
}

func TestInitRejectsBrokenSymlinkDestination(t *testing.T) {
	d := initRepo(t)
	path := filepath.Join(d, ".pitstop.json")
	if err := os.Symlink(filepath.Join(d, "missing"), path); err != nil {
		t.Fatal(err)
	}
	_, err := Init(context.Background(), d, InitOptions{Preset: "base", Architecture: "linux/amd64"})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != "CONFIG_EXISTS" {
		t.Fatalf("error=%v", err)
	}
	if _, lstatErr := os.Lstat(path); lstatErr != nil {
		t.Fatalf("broken symlink was changed: %v", lstatErr)
	}
}

func TestInitAllPresetsProduceParseableConfigs(t *testing.T) {
	for _, preset := range agentassets.PresetNames() {
		t.Run(preset, func(t *testing.T) {
			d := t.TempDir()
			result, err := Init(context.Background(), d, InitOptions{Preset: preset, Architecture: "linux/amd64"})
			if err != nil || !result.Written {
				t.Fatalf("result=%#v error=%v", result, err)
			}
			if _, err := Load(d); err != nil {
				t.Fatalf("generated config did not parse: %v", err)
			}
		})
	}
	t.Run("custom", func(t *testing.T) {
		d := t.TempDir()
		result, err := Init(context.Background(), d, InitOptions{Preset: "custom", Image: "alpine:3.22", Commands: []string{"printf '✓\\n'"}, Architecture: "linux/amd64"})
		if err != nil || !result.Written {
			t.Fatalf("result=%#v error=%v", result, err)
		}
		if _, err := Load(d); err != nil {
			t.Fatalf("generated config did not parse: %v", err)
		}
	})
}

func TestInitConcurrentCreators(t *testing.T) {
	d := initRepo(t)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := Init(context.Background(), d, InitOptions{Preset: "base", Architecture: "linux/amd64"})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	var success, exists int
	for err := range results {
		if err == nil {
			success++
			continue
		}
		var me *model.Error
		if errors.As(err, &me) && me.Code == "CONFIG_EXISTS" {
			exists++
		} else {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if success != 1 || exists != 1 {
		t.Fatalf("success=%d exists=%d", success, exists)
	}
	if cfg, err := Load(d); err != nil || cfg.Checks[0].Steps[0].Run == "" {
		t.Fatalf("winner wrote incomplete config: %#v, %v", cfg, err)
	}
}

func TestInitCancellationPreservesReplacement(t *testing.T) {
	d := initRepo(t)
	path := filepath.Join(d, ".pitstop.json")
	ctx := &replaceOnCancelContext{Context: context.Background(), path: path}
	_, err := Init(ctx, d, InitOptions{Preset: "base", Architecture: "linux/amd64"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "replacement" {
		t.Fatalf("replacement was removed: %q, %v", got, readErr)
	}
}

func TestInitCancellationRetainsInspectableFile(t *testing.T) {
	d := initRepo(t)
	ctx := &cancelAfterOpenContext{Context: context.Background()}
	result, err := Init(ctx, d, InitOptions{Preset: "base", Architecture: "linux/amd64"})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != "CANCELLED" || result.Written || !strings.Contains(me.Message, initRecoveryMessage) {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	if _, statErr := os.Stat(filepath.Join(d, ".pitstop.json")); statErr != nil {
		t.Fatalf("cancelled init did not retain destination: %v", statErr)
	}
	_, retryErr := Init(context.Background(), d, InitOptions{Preset: "base", Architecture: "linux/amd64"})
	if !errors.As(retryErr, &me) || me.Code != "CONFIG_EXISTS" {
		t.Fatalf("retry error=%v", retryErr)
	}
}

func TestInitWriteFailuresRetainInspectableFile(t *testing.T) {
	tests := []struct {
		name string
		set  func()
	}{
		{name: "write", set: func() { initWriteFile = func(*os.File, []byte) error { return errors.New("write failed") } }},
		{name: "sync", set: func() { initSyncFile = func(*os.File) error { return errors.New("sync failed") } }},
		{name: "close", set: func() { initCloseFile = func(f *os.File) error { _ = f.Close(); return errors.New("close failed") } }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			oldWrite, oldSync, oldClose := initWriteFile, initSyncFile, initCloseFile
			t.Cleanup(func() { initWriteFile, initSyncFile, initCloseFile = oldWrite, oldSync, oldClose })
			test.set()
			d := initRepo(t)
			result, err := Init(context.Background(), d, InitOptions{Preset: "base", Architecture: "linux/amd64"})
			var me *model.Error
			if !errors.As(err, &me) || me.Code != "WRITE_FAILED" || result.Written || !strings.Contains(me.Message, initRecoveryMessage) {
				t.Fatalf("result=%#v error=%v", result, err)
			}
			if _, statErr := os.Stat(filepath.Join(d, ".pitstop.json")); statErr != nil {
				t.Fatalf("failed init did not retain destination: %v", statErr)
			}
			_, retryErr := Init(context.Background(), d, InitOptions{Preset: "base", Architecture: "linux/amd64"})
			if !errors.As(retryErr, &me) || me.Code != "CONFIG_EXISTS" {
				t.Fatalf("retry error=%v", retryErr)
			}
		})
	}
}

func TestInitWriteFailurePreservesReplacement(t *testing.T) {
	d := initRepo(t)
	path := filepath.Join(d, ".pitstop.json")
	oldWrite := initWriteFile
	t.Cleanup(func() { initWriteFile = oldWrite })
	initWriteFile = func(f *os.File, _ []byte) error {
		if err := f.Close(); err != nil {
			return err
		}
		if err := os.Rename(path, path+".old"); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
			return err
		}
		return errors.New("controlled write failure")
	}
	result, err := Init(context.Background(), d, InitOptions{Preset: "base", Architecture: "linux/amd64"})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != "WRITE_FAILED" || result.Written || !strings.Contains(me.Message, initRecoveryMessage) {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "replacement" {
		t.Fatalf("replacement was removed: %q, %v", got, readErr)
	}
}
