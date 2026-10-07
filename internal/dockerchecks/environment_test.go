package dockerchecks

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCommandPinsEndpoint(t *testing.T) {
	t.Setenv("DOCKER_CONTEXT", "other")
	env := Environment{DockerPath: "/fake/docker", DockerHost: "unix:///chosen.sock"}
	cmd := command(context.Background(), env, "version")
	if !reflect.DeepEqual(cmd.Args, []string{"/fake/docker", "--host", "unix:///chosen.sock", "version"}) {
		t.Fatal(cmd.Args)
	}
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "DOCKER_CONTEXT=") {
			t.Fatal(e)
		}
	}
}
func TestCleanupRejectsInvalidOwnership(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "engine-meta.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Cleanup(context.Background(), d); err == nil {
		t.Fatal("invalid ownership metadata accepted")
	}
}
