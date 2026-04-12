package vulpinemark

import "testing"

func TestSetElementFilter_OnlyButtons(t *testing.T) {
	m := &Mark{}
	// Default: no filter.
	if f := m.currentFilter(); f != nil {
		t.Error("new Mark should have nil filter")
	}

	m.SetElementFilter(IncludeRoles("button"))
	f := m.currentFilter()
	if f == nil {
		t.Fatal("filter not set")
	}

	cases := []struct {
		el   Element
		keep bool
	}{
		{Element{Role: "button"}, true},
		{Element{Role: "BUTTON"}, true},
		{Element{Role: "link"}, false},
		{Element{Role: "input"}, false},
		{Element{Role: "checkbox"}, false},
	}
	for _, c := range cases {
		if got := f(c.el); got != c.keep {
			t.Errorf("filter(%q) = %v, want %v", c.el.Role, got, c.keep)
		}
	}

	// Remove filter.
	m.SetElementFilter(nil)
	if f := m.currentFilter(); f != nil {
		t.Error("filter not cleared")
	}
}

func TestExcludeRoles(t *testing.T) {
	f := ExcludeRoles("checkbox", "radio")
	if f(Element{Role: "button"}) != true {
		t.Error("button should pass")
	}
	if f(Element{Role: "checkbox"}) != false {
		t.Error("checkbox should be excluded")
	}
	if f(Element{Role: "RADIO"}) != false {
		t.Error("RADIO (uppercase) should be excluded")
	}
}

func TestCombineFilters(t *testing.T) {
	include := IncludeRoles("button", "link")
	exclude := ExcludeRoles("link")
	combined := combineFilters(include, exclude)
	if combined == nil {
		t.Fatal("combined nil")
	}
	if !combined(Element{Role: "button"}) {
		t.Error("button should pass combined")
	}
	if combined(Element{Role: "link"}) {
		t.Error("link should be excluded by combined")
	}
	if combined(Element{Role: "input"}) {
		t.Error("input should be excluded by combined (not in include)")
	}

	// All nil -> nil.
	if combineFilters(nil, nil) != nil {
		t.Error("all nil should combine to nil")
	}

	// Empty include.
	if IncludeRoles() != nil {
		t.Error("empty IncludeRoles should be nil")
	}
	if IncludeRoles("", "  ") != nil {
		t.Error("whitespace-only roles should yield nil")
	}
}
