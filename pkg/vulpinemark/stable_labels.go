package vulpinemark

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

// stableLabelBucketPx is the coordinate quantization step used when
// computing stable label hashes. Small pixel jitter within this
// bucket produces the same label so re-renders of the same page keep
// labels steady even if layout shifts a few pixels.
const stableLabelBucketPx = 32

// stableLabelModulus is the prime bound on hashed label numbers. It
// must be prime so that the linear-probe offset (which is < this
// modulus) is guaranteed coprime with it and will visit every slot
// before wrapping. 9973 is the largest prime below 10000 so labels
// remain at most 4 digits.
const stableLabelModulus = 9973

// floorBucket returns floor(v/step)*step. Unlike roundTo this is
// stable across small negative jitter at bucket edges.
func floorBucket(v float64, step int) int {
	if step <= 0 {
		return int(v)
	}
	n := int(v) / step
	if int(v) < 0 && int(v)%step != 0 {
		n--
	}
	return n * step
}

// stableLabelFor returns a deterministic "@N" label derived from the
// element's semantic identity (role, accessible name, rounded x/y).
// Because the input space is small, collisions are possible; the
// caller must pass a collisions map to reserve unique numbers across
// a single annotate pass. On collision the label falls back to the
// next free integer starting from the preferred hash bucket.
func stableLabelFor(e Element, used map[int]struct{}) string {
	h := sha1.New()
	h.Write([]byte(e.Role))
	h.Write([]byte{0})
	h.Write([]byte(e.Text))
	h.Write([]byte{0})
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(floorBucket(e.X, stableLabelBucketPx)))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(floorBucket(e.Y, stableLabelBucketPx)))
	h.Write(buf[:])

	sum := h.Sum(nil)
	// Mod into a friendly 1..stableLabelModulus range so labels stay
	// short for typical pages. The modulus is prime so every linear
	// probe offset in [1, modulus) is coprime and can visit every
	// slot before wrapping — otherwise a worst-case offset sharing a
	// factor with the modulus would visit only (modulus/gcd) slots.
	n := int(binary.BigEndian.Uint32(append([]byte{0}, sum[:3]...))) % stableLabelModulus
	if n < 1 {
		n = 1
	}
	// Linear probe into the used set so two elements that hash to the
	// same bucket still get distinct labels. The offset comes from
	// the next digest byte; since stableLabelModulus is prime, every
	// nonzero offset mod the modulus is coprime with it.
	offset := int(sum[3]%7) + 1
	for i := 0; i < stableLabelModulus; i++ {
		cand := ((n-1+i*offset)%stableLabelModulus + 1)
		if _, taken := used[cand]; !taken {
			used[cand] = struct{}{}
			return fmt.Sprintf("@%d", cand)
		}
	}
	// Extremely unlikely: fall back to sequential.
	for i := 1; ; i++ {
		if _, taken := used[i]; !taken {
			used[i] = struct{}{}
			return fmt.Sprintf("@%d", i)
		}
	}
}

// UseStableLabels enables semantic-hash labels for subsequent annotate
// calls. When enabled, labels are derived from the element's role,
// accessible name, and bucketed coordinates instead of its index in
// the enumeration order. The same element keeps the same label across
// re-renders as long as its role, name, and rough position are
// unchanged. Default: off.
func (m *Mark) UseStableLabels(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stableLabels = on
}

// assignStableLabels overwrites el[i].Label for every element using
// stableLabelFor and a shared collision set.
func assignStableLabels(els []Element) {
	used := make(map[int]struct{}, len(els))
	for i := range els {
		els[i].Label = stableLabelFor(els[i], used)
	}
}
