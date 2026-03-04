package provider

import (
	"context"
	"testing"

	"metadata-service/internal/model"
)

// stubProvider is a minimal Provider implementation for testing.
type stubProvider struct {
	name string
}

func (s *stubProvider) Name() string                                              { return s.name }
func (s *stubProvider) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	return nil, nil
}
func (s *stubProvider) GetWork(_ context.Context, _ string) (*model.Work, error) { return nil, nil }
func (s *stubProvider) GetEditions(_ context.Context, _ string) ([]model.Edition, error) {
	return nil, nil
}
func (s *stubProvider) ResolveIdentifier(_ context.Context, _, _ string) (*model.Edition, error) {
	return nil, nil
}

func TestRegistry_PriorityOrdering(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterWithConfig(&stubProvider{"slow"}, 30, true)
	reg.RegisterWithConfig(&stubProvider{"fast"}, 10, true)
	reg.RegisterWithConfig(&stubProvider{"mid"}, 20, true)

	got := reg.EnabledProviders()
	if len(got) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(got))
	}
	want := []string{"fast", "mid", "slow"}
	for i, p := range got {
		if p.Name() != want[i] {
			t.Errorf("position %d: want %q, got %q", i, want[i], p.Name())
		}
	}
}

func TestRegistry_SamePriorityStable(t *testing.T) {
	// All enabled providers should appear even when priorities are equal.
	reg := NewRegistry()
	reg.RegisterWithConfig(&stubProvider{"a"}, 10, true)
	reg.RegisterWithConfig(&stubProvider{"b"}, 10, true)

	got := reg.EnabledProviders()
	if len(got) != 2 {
		t.Errorf("expected 2 providers, got %d", len(got))
	}
}

func TestRegistry_SetEnabled_Disable(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterWithConfig(&stubProvider{"alpha"}, 1, true)
	reg.RegisterWithConfig(&stubProvider{"beta"}, 2, true)

	reg.SetEnabled("alpha", false)

	got := reg.EnabledProviders()
	if len(got) != 1 {
		t.Fatalf("expected 1 provider after disable, got %d", len(got))
	}
	if got[0].Name() != "beta" {
		t.Errorf("expected beta, got %s", got[0].Name())
	}
}

func TestRegistry_SetEnabled_Reenable(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterWithConfig(&stubProvider{"x"}, 1, false)

	if len(reg.EnabledProviders()) != 0 {
		t.Fatal("expected 0 enabled providers initially")
	}

	reg.SetEnabled("x", true)
	if len(reg.EnabledProviders()) != 1 {
		t.Fatal("expected 1 enabled provider after re-enable")
	}
}

func TestRegistry_SetEnabled_UnknownName(t *testing.T) {
	// SetEnabled on a non-existent provider must not panic.
	reg := NewRegistry()
	reg.SetEnabled("nonexistent", true)
}

func TestRegistry_AllProviders_IncludesDisabled(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterWithConfig(&stubProvider{"on"}, 1, true)
	reg.RegisterWithConfig(&stubProvider{"off"}, 2, false)

	all := reg.AllProviders()
	if len(all) != 2 {
		t.Errorf("AllProviders expected 2, got %d", len(all))
	}
	enabled := reg.EnabledProviders()
	if len(enabled) != 1 {
		t.Errorf("EnabledProviders expected 1, got %d", len(enabled))
	}
}

func TestRegistry_Register_DefaultsEnabledPriority100(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubProvider{"default"})

	all := reg.AllProviders()
	rp, ok := all["default"]
	if !ok {
		t.Fatal("provider not found")
	}
	if !rp.enabled {
		t.Error("default Register should set enabled=true")
	}
	if rp.priority != 100 {
		t.Errorf("default Register should set priority=100, got %d", rp.priority)
	}
}

func TestRegistry_Get(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubProvider{"myp"})

	p, ok := reg.Get("myp")
	if !ok || p.Name() != "myp" {
		t.Error("Get should return registered provider")
	}
	_, ok = reg.Get("missing")
	if ok {
		t.Error("Get should return false for unknown provider")
	}
}

func TestRegistry_SetPriority_ChangesDispatchOrder(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterWithConfig(&stubProvider{"first"}, 10, true)
	reg.RegisterWithConfig(&stubProvider{"second"}, 20, true)

	// Flip priorities at runtime
	reg.SetPriority("first", 99)
	reg.SetPriority("second", 1)

	got := reg.EnabledProviders()
	if got[0].Name() != "second" {
		t.Errorf("after SetPriority, expected 'second' first, got %q", got[0].Name())
	}
	if got[1].Name() != "first" {
		t.Errorf("after SetPriority, expected 'first' second, got %q", got[1].Name())
	}
}

func TestRegistry_SetPriority_UnknownName_NoPanic(t *testing.T) {
	reg := NewRegistry()
	reg.SetPriority("nonexistent", 5) // must not panic
}
