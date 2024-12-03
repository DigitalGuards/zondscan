package bitfield

type Bitfield []byte

func New(size uint) Bitfield {
	l, bits := indexOf(size)

	if bits > 0 {
		l += 1
	}

	return make([]byte, l)
}

// Set the n-th bit.
func (bf Bitfield) Set(n uint) {
	i, bit := indexOf(n)

	// Set the bit by using bitwise OR.
	bf[i] |= 1 << bit
}

// Clear the n-th bit.
func (bf Bitfield) Clear(n uint) {
	i, bit := indexOf(n)

	// Unset the bit using bitwise AND NOT.
	bf[i] &^= 1 << bit
}

// IsSet will return whether the n-th bit is set or not.
func (bf Bitfield) IsSet(n uint) bool {
	i, bit := indexOf(n)

	// Check if the bit is set by using bitwise AND to see if the result is non-zero.
	return bf[i]&(1<<bit) != 0
}

func indexOf(n uint) (uint, uint) {
	// Get the index of the byte and the offset of the bit within the byte of the bit the caller is interested in.
	// We can accomplish this by right shifting n with the log2 of 8 and the bit offset will be the 3 lower bits of n.

	return n >> 3, n & 7
}
