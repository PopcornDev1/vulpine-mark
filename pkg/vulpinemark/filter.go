package vulpinemark

import "strings"

// ElementFilter is a predicate applied to each enumerated element
// after visibility and occlusion checks. Returning false drops the
// element from the annotation.
type ElementFilter func(el Element) bool

// SetElementFilter installs a custom filter for subsequent Annotate
// calls. Pass nil to remove the filter. The filter runs after
// enumeration and visibility/occlusion checks but before label
// assignment and max-elements capping, so the cap is computed against
// the filtered set.
//
// Safe to call concurrently with in-flight Annotate calls: the next
// call observes the new filter, current calls use whatever was set
// when they read the field.
func (m *Mark) SetElementFilter(f ElementFilter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filter = f
}

// currentFilter returns the configured filter or nil.
func (m *Mark) currentFilter() ElementFilter {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.filter
}

// IncludeRoles returns an ElementFilter that keeps only elements
// whose role is in the given set. Case-insensitive. Returns nil if
// the set is empty (i.e. no filter).
func IncludeRoles(roles ...string) ElementFilter {
	if len(roles) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		r = strings.ToLower(strings.TrimSpace(r))
		if r == "" {
			continue
		}
		set[r] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return func(el Element) bool {
		_, ok := set[strings.ToLower(el.Role)]
		return ok
	}
}

// ExcludeRoles returns an ElementFilter that drops any element whose
// role is in the given set. Case-insensitive.
func ExcludeRoles(roles ...string) ElementFilter {
	if len(roles) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		r = strings.ToLower(strings.TrimSpace(r))
		if r == "" {
			continue
		}
		set[r] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return func(el Element) bool {
		_, ok := set[strings.ToLower(el.Role)]
		return !ok
	}
}

// combineFilters returns a filter that accepts only elements for which
// every non-nil input filter accepts. Returns nil if all inputs are nil.
func combineFilters(filters ...ElementFilter) ElementFilter {
	var active []ElementFilter
	for _, f := range filters {
		if f != nil {
			active = append(active, f)
		}
	}
	if len(active) == 0 {
		return nil
	}
	if len(active) == 1 {
		return active[0]
	}
	return func(el Element) bool {
		for _, f := range active {
			if !f(el) {
				return false
			}
		}
		return true
	}
}
