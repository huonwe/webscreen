package gamestream

import "errors"

// GF(2^8) arithmetic with primitive polynomial 0x11d (matching nanors).
var (
	gfExp [512]byte
	gfLog [256]byte
	gfInv [256]byte
)

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(x)
		gfExp[i+255] = byte(x)
		gfLog[x] = byte(i)
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11d
		}
	}
	gfInv[0] = 0
	for i := 1; i < 256; i++ {
		gfInv[i] = gfExp[255-int(gfLog[i])]
	}
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// rsDecode recovers missing data shards in-place.
// shards has length dataShards + parityShards, each shard having identical length shardLen.
// present[i] is true if shards[i] was received.
func rsDecode(shards [][]byte, present []bool, dataShards, parityShards, shardLen int) error {
	total := dataShards + parityShards
	if dataShards <= 0 || parityShards <= 0 || len(shards) != total || len(present) != total {
		return errors.New("rs: invalid shard parameters")
	}

	// Count available shards; need at least dataShards.
	avail := make([]int, 0, dataShards)
	missingData := false
	for i := 0; i < total; i++ {
		if present[i] {
			avail = append(avail, i)
		} else if i < dataShards {
			missingData = true
		}
	}
	if !missingData {
		return nil
	}
	if len(avail) < dataShards {
		return errors.New("rs: too many missing shards")
	}
	avail = avail[:dataShards]

	// Build K x K decoding matrix from the rows corresponding to avail,
	// augmented with the K x K identity matrix on the right.
	k := dataShards
	mat := make([][]byte, k)
	invMat := make([][]byte, k)
	for r := 0; r < k; r++ {
		mat[r] = make([]byte, k)
		invMat[r] = make([]byte, k)
		invMat[r][r] = 1

		idx := avail[r]
		if idx < k {
			mat[r][idx] = 1
		} else {
			pRow := idx - k
			for c := 0; c < k; c++ {
				mat[r][c] = gfInv[(parityShards+c)^pRow]
			}
		}
	}

	// Gauss-Jordan elimination over GF(2^8).
	for col := 0; col < k; col++ {
		pivot := -1
		for r := col; r < k; r++ {
			if mat[r][col] != 0 {
				pivot = r
				break
			}
		}
		if pivot < 0 {
			return errors.New("rs: singular matrix")
		}
		if pivot != col {
			mat[col], mat[pivot] = mat[pivot], mat[col]
			invMat[col], invMat[pivot] = invMat[pivot], invMat[col]
		}
		if mat[col][col] != 1 {
			inv := gfInv[mat[col][col]]
			for c := 0; c < k; c++ {
				mat[col][c] = gfMul(mat[col][c], inv)
				invMat[col][c] = gfMul(invMat[col][c], inv)
			}
		}
		for r := 0; r < k; r++ {
			if r == col || mat[r][col] == 0 {
				continue
			}
			factor := mat[r][col]
			for c := 0; c < k; c++ {
				mat[r][c] ^= gfMul(factor, mat[col][c])
				invMat[r][c] ^= gfMul(factor, invMat[col][c])
			}
		}
	}

	// Reconstruct only missing data shards.
	for d := 0; d < k; d++ {
		if present[d] {
			continue
		}
		dst := make([]byte, shardLen)
		for c := 0; c < k; c++ {
			coeff := invMat[d][c]
			if coeff == 0 {
				continue
			}
			src := shards[avail[c]]
			if coeff == 1 {
				for b := 0; b < shardLen; b++ {
					dst[b] ^= src[b]
				}
			} else {
				for b := 0; b < shardLen; b++ {
					dst[b] ^= gfMul(coeff, src[b])
				}
			}
		}
		shards[d] = dst
		present[d] = true
	}
	return nil
}
