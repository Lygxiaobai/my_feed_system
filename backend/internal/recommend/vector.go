package recommend

import (
	"encoding/binary"
	"math"
	"time"
)

func EncodeVector(v []float32) []byte {
	buf := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(x))
	}
	return buf
}

func decodeVector(b []byte, dim int) ([]float32, bool) {
	if dim <= 0 || len(b) != 4*dim {
		return nil, false
	}
	out := make([]float32, dim)
	for i := 0; i < dim; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out, true
}

func normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	inv := 1 / math.Sqrt(sum)
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) * inv)
	}
	return out
}

// cosine 假定两侧已归一化时退化为点积；未归一化时仍按定义计算。
func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func weightedMean(vectors [][]float32, weights []float64) []float32 {
	if len(vectors) == 0 || len(vectors) != len(weights) {
		return nil
	}
	dim := len(vectors[0])
	if dim == 0 {
		return nil
	}
	acc := make([]float64, dim)
	var wsum float64
	for i, vec := range vectors {
		if len(vec) != dim || weights[i] <= 0 {
			continue
		}
		wsum += weights[i]
		for j, x := range vec {
			acc[j] += float64(x) * weights[i]
		}
	}
	if wsum == 0 {
		return nil
	}
	out := make([]float32, dim)
	for i, x := range acc {
		out[i] = float32(x / wsum)
	}
	return normalize(out)
}

func timeDecay(at, now time.Time, halfLife time.Duration) float64 {
	if halfLife <= 0 {
		return 1
	}
	age := now.Sub(at).Seconds()
	if age <= 0 {
		return 1
	}
	return math.Exp(-math.Ln2 * age / halfLife.Seconds())
}
