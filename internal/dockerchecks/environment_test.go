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

func TestRemoveOwnedContainerRejectsChangedLabel(t *testing.T) {
	d := t.TempDir()
	marker := filepath.Join(d, "rm-called")
	docker := filepath.Join(d, "docker")
	script := "#!/bin/sh\ncase \"$*\" in\n  *inspect*) printf 'other\\n' ;;\n  *rm*) : > \"" + marker + "\" ;;\nesac\n"
	if err := os.WriteFile(docker, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	env := Environment{DockerPath: docker, DockerHost: "unix:///chosen.sock"}
	err := removeOwnedContainer(context.Background(), env, "container-id", "com.pitstop.run=expected")
	if err == nil {
		t.Fatal("ownership change was accepted")
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("container was removed after ownership changed")
	}
}

func TestImageReferenceUsesImmutableDigestFromTargetInspect(t *testing.T) {
	info := imageInspect{ID: "sha256:platform-config", RepoDigests: []string{"alpine@sha256:manifest"}}
	if got := imageReference(info, "alpine:latest"); got != "alpine@sha256:manifest" {
		t.Fatalf("image reference = %q", got)
	}
	if got := imageReference(imageInspect{ID: "sha256:custom"}, "custom:latest"); got != "sha256:custom" {
		t.Fatalf("custom image reference = %q", got)
	}
}
