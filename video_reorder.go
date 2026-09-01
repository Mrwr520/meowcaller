package meowcaller

// Video RTP reordering.
//
// H264AccessUnitAssembler is deliberately a strictly-ordered state machine: any
// sequence discontinuity means "this frame is damaged", it withholds the frame and
// asks for recovery (PLI). That contract is what its KAT proves, and it is the
// behaviour the PLI path depends on.
//
// The relay, however, does deliver video RTP out of order — observed on a live
// call as 1104, 1106, 1105 and 1107, 1109, 1108 within single frames. Feeding
// those straight into the assembler turns ordinary reordering into "loss": every
// swap discards a frame and fires a PLI, so the visible frame rate collapses.
//
// Reordering is a transport concern, so it is resolved here, before the assembler:
// packets are released in sequence order, and a missing packet is only declared
// lost once the buffer has accumulated videoReorderDepth newer packets. Real loss
// therefore still reaches the assembler as a gap (and still triggers PLI), while a
// swapped pair costs nothing.

// videoReorderDepth is how many newer packets may queue up before the packet we
// are waiting for is presumed lost. Video runs at a few packets per frame, so 8
// covers reordering across two or three frames without adding visible latency.
const videoReorderDepth = 8

// videoReorderPacket is one buffered RTP payload awaiting in-order release.
type videoReorderPacket struct {
	sequence uint16
	marker   bool
	payload  []byte
}

// videoReorderBuffer releases video RTP payloads in sequence order.
type videoReorderBuffer struct {
	pending  map[uint16]videoReorderPacket
	expected uint16
	started  bool
}

// push buffers one payload and returns everything that became deliverable, in
// sequence order. payload is copied: the caller owns the read buffer.
func (b *videoReorderBuffer) push(sequence uint16, marker bool, payload []byte) []videoReorderPacket {
	if b.pending == nil {
		b.pending = make(map[uint16]videoReorderPacket, videoReorderDepth*2)
	}
	if !b.started {
		b.started = true
		b.expected = sequence
	} else if seqOlder(sequence, b.expected) {
		// Already released (or given up on) this sequence number; a late copy of it
		// must not be pushed through the assembler as a discontinuity.
		return nil
	}
	if _, duplicate := b.pending[sequence]; duplicate {
		return nil
	}

	stored := make([]byte, len(payload))
	copy(stored, payload)
	b.pending[sequence] = videoReorderPacket{sequence: sequence, marker: marker, payload: stored}

	out := b.drain(nil)
	// Still blocked and the backlog is past the tolerance: treat the awaited packet
	// as lost, skip to the oldest packet we do have, and let the assembler see the
	// gap so it withholds the damaged frame and requests a keyframe.
	if len(b.pending) > videoReorderDepth {
		b.expected = b.oldestPending()
		out = b.drain(out)
	}
	return out
}

// drain appends every consecutive packet starting at expected.
func (b *videoReorderBuffer) drain(out []videoReorderPacket) []videoReorderPacket {
	for {
		packet, ok := b.pending[b.expected]
		if !ok {
			return out
		}
		delete(b.pending, b.expected)
		b.expected++
		out = append(out, packet)
	}
}

// oldestPending returns the lowest buffered sequence number in wrap-aware order.
func (b *videoReorderBuffer) oldestPending() uint16 {
	var oldest uint16
	first := true
	for sequence := range b.pending {
		if first || seqOlder(sequence, oldest) {
			oldest = sequence
			first = false
		}
	}
	return oldest
}

// seqOlder reports whether a precedes b in RTP sequence space (RFC 3550 wrap-aware).
func seqOlder(a, b uint16) bool {
	return int16(a-b) < 0
}
