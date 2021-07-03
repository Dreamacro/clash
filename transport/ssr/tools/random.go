package tools

import (
	"encoding/binary"

	"github.com/Dreamacro/clash/common/pool"
)

// XorShift128Plus - a pseudorandom number generator
type XorShift128Plus struct {
	s [2]uint64
}

func (r *XorShift128Plus) Next() uint64 {
	x := r.s[0]
	y := r.s[1]
	r.s[0] = y
	x ^= x << 23
	x ^= y ^ (x >> 17) ^ (y >> 26)
	r.s[1] = x
	return x + y
}

func (r *XorShift128Plus) InitFromBin(bin []byte) {
	var full []byte
	if len(bin) < 16 {
		full := pool.Get(16)[:0]
		defer pool.Put(full)
		full = append(full, bin...)
		for len(full) < 16 {
			full = append(full, 0)
		}
	} else {
		full = bin
	}
	r.s[0] = binary.LittleEndian.Uint64(full[:8])
	r.s[1] = binary.LittleEndian.Uint64(full[8:16])
}

func (r *XorShift128Plus) InitFromBinAndLength(bin []byte, length int) {
	var full []byte
	if len(bin) < 16 {
		full := pool.Get(16)[:0]
		defer pool.Put(full)
		full = append(full, bin...)
		for len(full) < 16 {
			full = append(full, 0)
		}
	}
	full = bin
	binary.LittleEndian.PutUint16(full, uint16(length))
	r.s[0] = binary.LittleEndian.Uint64(full[:8])
	r.s[1] = binary.LittleEndian.Uint64(full[8:16])
	for i := 0; i < 4; i++ {
		r.Next()
	}
}
