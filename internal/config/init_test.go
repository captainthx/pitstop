package config

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"pitstop/internal/model"
)

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
}
