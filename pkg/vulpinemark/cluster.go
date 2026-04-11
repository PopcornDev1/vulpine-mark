package vulpinemark

import (
	"fmt"
	"image"
	"sort"
	"strconv"
	"strings"
)

// minClusterSize is the minimum number of same-shape elements required
// to form a cluster. Groups with fewer members stay individually labeled.
const minClusterSize = 4

// clusterRoundPx is the rounding granularity (in CSS pixels) applied to
// element dimensions when computing the cluster grouping key. Elements
// whose width and height round to the same bucket are considered
// visually similar.
const clusterRoundPx = 8

// Cluster groups visually similar repeated elements (e.g. a product grid
// or a list of search results) under a single label like "@5". Individual
// members are addressed via "@5[1]", "@5[2]", ... using the existing
// Click/Type/Hover helpers.
type Cluster struct {
	// Label is the assigned cluster identifier, e.g. "@5".
	Label string `json:"label"`
	// Role is the shared semantic role of the members.
	Role string `json:"role"`
	// Members are the individual elements in reading order (top-to-bottom,
	// left-to-right).
	Members []Element `json:"members"`
	// BBox is the union of all member bounding rects in screenshot pixels.
	BBox image.Rectangle `json:"-"`
}

// clusterKey identifies a cluster candidate: role + rounded width/height.
type clusterKey struct {
	role string
	w    int
	h    int
}

func roundTo(v float64, step int) int {
	if step <= 0 {
		return int(v)
	}
	half := float64(step) / 2
	n := int((v + half) / float64(step))
	return n * step
}

// clusterElements groups visually similar repeated elements together.
// Elements that end up in a cluster are removed from the returned
// ungrouped slice. Groups with fewer than minClusterSize members are not
// formed and their elements stay ungrouped. The returned clusters are
// sorted by the reading-order position of their first member. Members
// inside each cluster are sorted top-to-bottom, left-to-right.
func clusterElements(els []Element) (clusters []Cluster, ungrouped []Element) {
	groups := make(map[clusterKey][]int)
	for i, el := range els {
		k := clusterKey{
			role: el.Role,
			w:    roundTo(el.Width, clusterRoundPx),
			h:    roundTo(el.Height, clusterRoundPx),
		}
		groups[k] = append(groups[k], i)
	}

	inCluster := make([]bool, len(els))
	// Iterate groups in a stable order so cluster numbering is
	// deterministic: first by rounded-y of the first member, then x.
	type entry struct {
		key clusterKey
		idx []int
	}
	entries := make([]entry, 0, len(groups))
	for k, idx := range groups {
		if len(idx) < minClusterSize {
			continue
		}
		entries = append(entries, entry{key: k, idx: idx})
	}
	sort.Slice(entries, func(i, j int) bool {
		ai := entries[i].idx[0]
		aj := entries[j].idx[0]
		if els[ai].Y != els[aj].Y {
			return els[ai].Y < els[aj].Y
		}
		return els[ai].X < els[aj].X
	})

	for n, e := range entries {
		members := make([]Element, 0, len(e.idx))
		for _, i := range e.idx {
			inCluster[i] = true
			members = append(members, els[i])
		}
		sort.SliceStable(members, func(i, j int) bool {
			if members[i].Y != members[j].Y {
				return members[i].Y < members[j].Y
			}
			return members[i].X < members[j].X
		})
		cl := Cluster{
			Label:   "@" + strconv.Itoa(n+1),
			Role:    e.key.role,
			Members: members,
			BBox:    clusterBBox(members, 1.0),
		}
		clusters = append(clusters, cl)
	}

	ungrouped = make([]Element, 0, len(els))
	for i, el := range els {
		if inCluster[i] {
			continue
		}
		ungrouped = append(ungrouped, el)
	}
	return clusters, ungrouped
}

// clusterBBox returns the union of all member rects scaled by `scale`
// (CSS px -> screenshot px).
func clusterBBox(members []Element, scale float64) image.Rectangle {
	if len(members) == 0 {
		return image.Rectangle{}
	}
	first := members[0]
	r := image.Rect(
		int(first.X*scale),
		int(first.Y*scale),
		int((first.X+first.Width)*scale),
		int((first.Y+first.Height)*scale),
	)
	for _, m := range members[1:] {
		mr := image.Rect(
			int(m.X*scale),
			int(m.Y*scale),
			int((m.X+m.Width)*scale),
			int((m.Y+m.Height)*scale),
		)
		r = r.Union(mr)
	}
	return r
}

// parseClusterRef parses a label of the form "@N[K]" into the cluster
// label "@N" and 1-based member index K. Returns ok=false for plain
// labels like "@7".
func parseClusterRef(label string) (clusterLabel string, memberIdx int, ok bool) {
	lb := strings.IndexByte(label, '[')
	if lb < 0 {
		return "", 0, false
	}
	if !strings.HasSuffix(label, "]") {
		return "", 0, false
	}
	inside := label[lb+1 : len(label)-1]
	n, err := strconv.Atoi(inside)
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return label[:lb], n, true
}

// ErrClusterIndexOutOfRange is returned when a cluster reference like
// "@5[9]" points to a member index beyond the cluster's size.
var ErrClusterIndexOutOfRange = fmt.Errorf("vulpinemark: cluster member index out of range")
