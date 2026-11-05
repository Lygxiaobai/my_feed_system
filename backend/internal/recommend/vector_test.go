package recommend

import (
	"math"
	"testing"
	"time"
)

func TestCosineOrthogonalAndParallel(t *testing.T) {
	if got := cosine([]float32{1, 0}, []float32{0, 1}); math.Abs(got) > 1e-6 {
		t.Fatalf("orthogonal cosine=%v", got)
	}
	if got := cosine([]float32{1, 0}, []float32{2, 0}); math.Abs(got-1) > 1e-5 {
		t.Fatalf("parallel cosine=%v", got)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := normalize([]float32{3, 4})
	raw := EncodeVector(in)
	out, ok := decodeVector(raw, 2)
	if !ok {
		t.Fatal("decode failed")
	}
	if math.Abs(float64(out[0]-in[0])) > 1e-6 || math.Abs(float64(out[1]-in[1])) > 1e-6 {
		t.Fatalf("roundtrip %v vs %v", out, in)
	}
}

func TestBuildUserVectorPrefersTip(t *testing.T) {
	now := time.Now()
	likeVec := normalize([]float32{1, 0})
	tipVec := normalize([]float32{0, 1})
	signals := []interestSignal{
		{VideoID: 1, Weight: likeWeight, At: now},
		{VideoID: 2, Weight: tipWeight, At: now},
	}
	emb := map[uint64][]float32{1: likeVec, 2: tipVec}
	got := buildUserVector(signals, emb, now)
	if got[1] <= got[0] {
		t.Fatalf("tip should dominate user vector, got %v", got)
	}
}

func TestEmbedTextJoinsTitleAndDescription(t *testing.T) {
	if EmbedText("标题", "描述") != "标题\n\n描述" {
		t.Fatal(EmbedText("标题", "描述"))
	}
	if EmbedText("标题", "  ") != "标题" {
		t.Fatal("empty desc")
	}
}
