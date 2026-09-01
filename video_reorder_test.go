package meowcaller

import "testing"

func sequences(packets []videoReorderPacket) []uint16 {
	out := make([]uint16, 0, len(packets))
	for _, p := range packets {
		out = append(out, p.sequence)
	}
	return out
}

func equalSequences(got []uint16, want ...uint16) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestVideoReorderReleasesInOrder(t *testing.T) {
	var b videoReorderBuffer
	if got := sequences(b.push(100, false, []byte{1})); !equalSequences(got, 100) {
		t.Fatalf("first packet released %v, want [100]", got)
	}
	if got := sequences(b.push(101, true, []byte{2})); !equalSequences(got, 101) {
		t.Fatalf("in-order packet released %v, want [101]", got)
	}
}

func TestVideoReorderAbsorbsSwappedPair(t *testing.T) {
	// The exact pattern seen on a live call: 1104, 1106, 1105.
	var b videoReorderBuffer
	if got := sequences(b.push(1104, false, []byte{1})); !equalSequences(got, 1104) {
		t.Fatalf("1104 released %v, want [1104]", got)
	}
	// 1106 arrives early: nothing may be released, 1105 is still expected.
	if got := sequences(b.push(1106, true, []byte{3})); len(got) != 0 {
		t.Fatalf("early 1106 released %v, want nothing", got)
	}
	// 1105 fills the hole; both come out in order.
	if got := sequences(b.push(1105, false, []byte{2})); !equalSequences(got, 1105, 1106) {
		t.Fatalf("filled hole released %v, want [1105 1106]", got)
	}
}

func TestVideoReorderDeclaresLossAfterDepth(t *testing.T) {
	var b videoReorderBuffer
	b.push(10, false, []byte{0})
	// 11 never arrives. Newer packets queue up without being released...
	for seq := uint16(12); seq <= 12+videoReorderDepth-1; seq++ {
		if got := sequences(b.push(seq, false, []byte{1})); len(got) != 0 {
			t.Fatalf("packet %d released %v while blocked, want nothing", seq, got)
		}
	}
	// ...until the backlog exceeds the tolerance, at which point 11 is presumed
	// lost and everything buffered is handed over starting at 12. The assembler
	// then sees 10 -> 12 as a gap and requests a keyframe.
	released := sequences(b.push(12+videoReorderDepth, false, []byte{1}))
	if len(released) == 0 || released[0] != 12 {
		t.Fatalf("after loss released %v, want a run starting at 12", released)
	}
}

func TestVideoReorderDropsAlreadyReleasedSequence(t *testing.T) {
	var b videoReorderBuffer
	b.push(50, false, []byte{1})
	b.push(51, true, []byte{2})
	// A late duplicate of an already-released packet must not be re-emitted:
	// the assembler would read it as a backwards discontinuity.
	if got := sequences(b.push(50, false, []byte{1})); len(got) != 0 {
		t.Fatalf("late duplicate released %v, want nothing", got)
	}
}

func TestVideoReorderHandlesSequenceWraparound(t *testing.T) {
	var b videoReorderBuffer
	if got := sequences(b.push(65534, false, []byte{1})); !equalSequences(got, 65534) {
		t.Fatalf("released %v, want [65534]", got)
	}
	// 0 arrives before 65535; wrap-aware ordering must still hold them correctly.
	if got := sequences(b.push(0, true, []byte{3})); len(got) != 0 {
		t.Fatalf("early wrapped packet released %v, want nothing", got)
	}
	if got := sequences(b.push(65535, false, []byte{2})); !equalSequences(got, 65535, 0) {
		t.Fatalf("wrapped release %v, want [65535 0]", got)
	}
}

func TestVideoReorderCopiesPayload(t *testing.T) {
	var b videoReorderBuffer
	scratch := []byte{9, 9, 9}
	released := b.push(1, true, scratch)
	scratch[0] = 0 // caller reuses its read buffer
	if len(released) != 1 || released[0].payload[0] != 9 {
		t.Fatalf("payload was not copied: %v", released)
	}
}
