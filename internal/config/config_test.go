package config

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"pitstop/internal/model"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const validConfig = `{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`

func TestLoadStrictAndMigration(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, ".pitstop.json")
	if err := os.WriteFile(path, []byte(validConfig), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(validConfig+"\n{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err == nil {
		t.Fatal("expected trailing document rejection")
	}
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"workflow":"ci.yml"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err == nil {
		t.Fatal("expected migration error")
	} else {
		var me *model.Error
		if !errors.As(err, &me) || me.Code != "CONFIG_INVALID" || !strings.Contains(me.Message, "schemaVersion 2") {
			t.Fatal(err)
		}
	}
}
func TestLoadRejectsUnknownFields(t *testing.T) {
	d := t.TempDir()
	b := []byte(`{"schemaVersion":2,"workflow":"ci.yml","runner":{"image":"x","architecture":"linux/arm64"},"jobs":["x"],"checks":[{"id":"x","steps":[{"id":"s","run":"true"}]}]}`)
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestLoadRejectsSpecialFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file types are outside the supported platform promise")
	}
	t.Run("missing", func(t *testing.T) {
		if _, err := Load(t.TempDir()); err == nil {
			t.Fatal("missing config accepted")
		}
	})
	t.Run("directory", func(t *testing.T) {
		d := t.TempDir()
		if err := os.Mkdir(filepath.Join(d, ".pitstop.json"), 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(d); err == nil {
			t.Fatal("directory accepted")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		d := t.TempDir()
		target := filepath.Join(d, "target")
		if err := os.WriteFile(target, []byte(validConfig), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(d, ".pitstop.json")); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(d); err == nil {
			t.Fatal("symlink accepted")
		}
	})
	t.Run("broken symlink", func(t *testing.T) {
		d := t.TempDir()
		if err := os.Symlink(filepath.Join(d, "missing"), filepath.Join(d, ".pitstop.json")); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(d); err == nil {
			t.Fatal("broken symlink accepted")
		}
	})
	t.Run("unreadable regular file", func(t *testing.T) {
		d := t.TempDir()
		path := filepath.Join(d, ".pitstop.json")
		if err := os.WriteFile(path, []byte(validConfig), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0000); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(path, 0600)
		if _, err := Load(d); err == nil {
			if os.Geteuid() == 0 {
				t.Skip("root bypasses regular-file permission bits")
			}
			t.Fatal("unreadable regular file accepted")
		}
	})
	t.Run("fifo", func(t *testing.T) {
		d := t.TempDir()
		path := filepath.Join(d, ".pitstop.json")
		if err := syscall.Mkfifo(path, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLoadFIFOHelper$")
		cmd.Env = append(os.Environ(), "PITSTOP_FIFO_REPO="+d)
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				t.Fatalf("FIFO read did not return before deadline: %v", ctx.Err())
			}
			t.Fatalf("FIFO helper failed: %v", err)
		}
	})
	t.Run("socket", func(t *testing.T) {
		d, err := os.MkdirTemp("/tmp", "ps-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(d)
		path := filepath.Join(d, ".pitstop.json")
		listener, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		if _, err := Load(d); err == nil {
			t.Fatal("socket accepted")
		}
	})
	t.Run("concurrent symlink replacement", func(t *testing.T) {
		d := t.TempDir()
		outsideDir := t.TempDir()
		outside := filepath.Join(outsideDir, "outside.json")
		outsideConfig := strings.Replace(validConfig, `"run":"true"`, `"run":"outside"`, 1)
		if err := os.WriteFile(outside, []byte(outsideConfig), 0600); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(d, ".pitstop.json")
		if err := os.WriteFile(path, []byte(validConfig), 0600); err != nil {
			t.Fatal(err)
		}
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tmp := filepath.Join(d, ".pitstop.tmp")
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = os.WriteFile(tmp, []byte(validConfig), 0600)
				_ = os.Rename(tmp, path)
				_ = os.Remove(path)
				_ = os.Symlink(outside, path)
			}
		}()
		for i := 0; i < 200; i++ {
			c, err := Load(d)
			if err == nil && c.Checks[0].Steps[0].Run != "true" {
				t.Fatalf("read outside contents during replacement: %q", c.Checks[0].Steps[0].Run)
			}
		}
		close(stop)
		wg.Wait()
	})
}

func TestLoadFIFOHelper(t *testing.T) {
	repo := os.Getenv("PITSTOP_FIFO_REPO")
	if repo == "" {
		return
	}
	if _, err := Load(repo); err == nil {
		t.Fatal("FIFO accepted")
	}
}

func TestLoadBoundsOpenedFile(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(strings.Repeat("x", model.MaxConfigBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err == nil {
		t.Fatal("oversized config accepted")
	}
}

func TestOverridesMatchConfiguredSelection(t *testing.T) {
	c := model.Config{SchemaVersion: 2, Jobs: []string{"verify", "lint"}, Runner: model.RunnerConfig{Image: "node:22-bookworm", Architecture: "linux/arm64"}, Checks: []model.Check{{ID: "verify", Steps: []model.CheckStep{{ID: "s", Run: "true"}}}, {ID: "lint", Steps: []model.CheckStep{{ID: "s", Run: "true"}}}}}
	originalJobs := append([]string(nil), c.Jobs...)
	got, err := ApplyOverrides(c, []string{"lint"}, "linux/amd64")
	if err != nil || len(got.Jobs) != 1 || got.Jobs[0] != "lint" || got.Runner.Architecture != "linux/amd64" {
		t.Fatalf("apply overrides = %#v, %v", got, err)
	}
	if strings.Join(c.Jobs, ",") != strings.Join(originalJobs, ",") {
		t.Fatal("ApplyOverrides mutated input jobs")
	}
	for _, jobs := range [][]string{{"unknown"}, {"verify", "verify"}} {
		if _, err := ApplyOverrides(c, jobs, ""); err == nil {
			t.Fatalf("accepted invalid selection %v", jobs)
		}
	}
	for _, arch := range []string{"", "linux/arm64", "linux/amd64"} {
		if got, err := ApplyOverrides(c, nil, arch); err != nil || got.Runner.Architecture != map[string]string{"": "linux/arm64", "linux/arm64": "linux/arm64", "linux/amd64": "linux/amd64"}[arch] {
			t.Fatalf("architecture %q: %#v, %v", arch, got, err)
		}
	}
	for _, arch := range []string{"darwin/arm64", "linux/386"} {
		if _, err := ApplyOverrides(c, nil, arch); err == nil {
			t.Fatalf("accepted invalid architecture %q", arch)
		}
	}
}
