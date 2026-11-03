package cgroup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tests use plain directories: writing a file there behaves like
// writing a cgroup file that accepts any value.

func TestNewCreatesParentAndGroup(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "resource-lab")
	g, err := New(parent, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if g.Dir != filepath.Join(parent, "run-1") {
		t.Errorf("Dir = %q", g.Dir)
	}
	data, err := os.ReadFile(filepath.Join(parent, "cgroup.subtree_control"))
	if err != nil || string(data) != "+cpu +memory" {
		t.Errorf("subtree_control = %q, %v; want \"+cpu +memory\"", data, err)
	}
	if _, err := New(parent, "run-2"); err != nil {
		t.Errorf("second group in the same parent: %v", err)
	}
	if _, err := New(parent, "run-1"); err == nil {
		t.Error("creating run-1 twice returned no error")
	}
}

func TestApplyWritesAndVerifies(t *testing.T) {
	g := &Group{Dir: t.TempDir()}
	results := g.Apply(Limits{CPUPercent: 50, MemoryBytes: 128 << 20, SwapBytes: 0})
	want := map[string]string{
		"cpu.max":         "50000 100000",
		"memory.max":      "134217728",
		"memory.swap.max": "0",
	}
	for file, value := range want {
		data, err := os.ReadFile(filepath.Join(g.Dir, file))
		if err != nil || string(data) != value {
			t.Errorf("%s = %q, %v; want %q", file, data, err, value)
		}
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for _, r := range results {
		if !r.Applied {
			t.Errorf("%s not applied: %v", r.Name, r.Err)
		}
	}
}

func TestApplyOnlyRequestedLimits(t *testing.T) {
	g := &Group{Dir: t.TempDir()}
	if results := g.Apply(Limits{SwapBytes: -1}); len(results) != 0 {
		t.Errorf("no limits requested, got %v", results)
	}
	if _, err := os.Stat(filepath.Join(g.Dir, "cpu.max")); err == nil {
		t.Error("cpu.max was written although no CPU limit was requested")
	}
}

func TestApplyReportsFailure(t *testing.T) {
	g := &Group{Dir: filepath.Join(t.TempDir(), "missing")}
	results := g.Apply(Limits{CPUPercent: 25, SwapBytes: -1})
	if len(results) != 1 || results[0].Applied || results[0].Err == nil {
		t.Fatalf("results = %v, want one failed cpu limit", results)
	}
	if s := results[0].String(); !strings.Contains(s, "NOT applied") {
		t.Errorf("String() = %q, want it to say NOT applied", s)
	}
}

func TestMemoryLimitIsPageAligned(t *testing.T) {
	g := &Group{Dir: t.TempDir()}
	g.Apply(Limits{MemoryBytes: 100<<20 + 123, SwapBytes: -1})
	data, _ := os.ReadFile(filepath.Join(g.Dir, "memory.max"))
	if string(data) != "104857600" {
		t.Errorf("memory.max = %q, want 104857600", data)
	}
}

func TestNotApplied(t *testing.T) {
	cause := errors.New("no cgroup")
	results := Limits{CPUPercent: 100, MemoryBytes: 64 << 20, SwapBytes: -1}.NotApplied(cause)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for _, r := range results {
		if r.Applied || !errors.Is(r.Err, cause) {
			t.Errorf("%s: Applied %v, Err %v", r.Name, r.Applied, r.Err)
		}
	}
}

func TestRemoveEmptyAndMissing(t *testing.T) {
	g := &Group{Dir: filepath.Join(t.TempDir(), "run")}
	if err := os.Mkdir(g.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := g.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := g.Remove(); err != nil {
		t.Errorf("removing a missing group: %v", err)
	}
}
