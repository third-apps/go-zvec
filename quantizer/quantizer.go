package quantizer

import (
	"encoding/binary"

	"fmt"
	"math"
	"math/rand"

	"github.com/third-apps/go-zvec/types"
)

type Quantizer interface {
	Encode(vec []float32, dst []byte) []byte
	Decode(src []byte, dst []float32) []float32
	CodeSize() int
	Type() types.QuantizeType
}

type FP16Quantizer struct{}

func NewFP16Quantizer() *FP16Quantizer {
	return &FP16Quantizer{}
}

func (q *FP16Quantizer) Type() types.QuantizeType {
	return types.QuantizeTypeFP16
}

func (q *FP16Quantizer) CodeSize() int {
	return 2
}

func (q *FP16Quantizer) Encode(vec []float32, dst []byte) []byte {
	needed := len(vec) * 2
	if cap(dst) < needed {
		dst = make([]byte, needed)
	} else {
		dst = dst[:needed]
	}
	for i, v := range vec {
		binary.LittleEndian.PutUint16(dst[i*2:], float32ToFloat16(v))
	}
	return dst
}

func (q *FP16Quantizer) Decode(src []byte, dst []float32) []float32 {
	n := len(src) / 2
	if cap(dst) < n {
		dst = make([]float32, n)
	} else {
		dst = dst[:n]
	}
	for i := 0; i < n; i++ {
		dst[i] = float16ToFloat32(binary.LittleEndian.Uint16(src[i*2:]))
	}
	return dst
}

type Int8Quantizer struct {
	dim       int
	scale     []float32
	rotMatrix [][]float32
	enableRot bool
	trained   bool
}

func NewInt8Quantizer(dim int, enableRot bool) *Int8Quantizer {
	return &Int8Quantizer{
		dim:       dim,
		enableRot: enableRot,
	}
}

func (q *Int8Quantizer) Type() types.QuantizeType {
	return types.QuantizeTypeInt8
}

func (q *Int8Quantizer) CodeSize() int {
	return q.dim
}

func (q *Int8Quantizer) Train(vectors [][]float32) {
	n := len(vectors)
	if n == 0 {
		return
	}

	if q.enableRot {
		q.rotMatrix = computeRandomRotation(q.dim, 42)
	}

	q.scale = make([]float32, q.dim)
	for j := 0; j < q.dim; j++ {
		var maxAbs float32
		for i := 0; i < n; i++ {
			v := vectors[i]
			if j < len(v) {
				abs := float32(math.Abs(float64(v[j])))
				if abs > maxAbs {
					maxAbs = abs
				}
			}
		}
		if maxAbs > 0 {
			q.scale[j] = maxAbs
		} else {
			q.scale[j] = 1.0
		}
	}
	q.trained = true
}

func (q *Int8Quantizer) Encode(vec []float32, dst []byte) []byte {
	needed := q.dim
	if cap(dst) < needed {
		dst = make([]byte, needed)
	} else {
		dst = dst[:needed]
	}

	v := vec
	if q.enableRot && q.rotMatrix != nil {
		v = applyRotation(vec, q.rotMatrix)
	}

	for j := 0; j < q.dim && j < len(v); j++ {
		var s float32
		if j < len(q.scale) && q.scale[j] != 0 {
			s = q.scale[j]
		} else {
			s = 1.0
		}
		scaled := v[j] / s
		if scaled > 1.0 {
			scaled = 1.0
		} else if scaled < -1.0 {
			scaled = -1.0
		}
		dst[j] = uint8(int8(scaled * 127.0))
	}
	return dst
}

func (q *Int8Quantizer) Decode(src []byte, dst []float32) []float32 {
	n := len(src)
	if cap(dst) < n {
		dst = make([]float32, n)
	} else {
		dst = dst[:n]
	}
	for j := 0; j < n; j++ {
		dst[j] = float32(int8(src[j])) / 127.0
		if j < len(q.scale) {
			dst[j] *= q.scale[j]
		}
	}
	if q.enableRot && q.rotMatrix != nil {
		dst = applyInverseRotation(dst, q.rotMatrix)
	}
	return dst
}

type Int4Quantizer struct {
	dim       int
	scale     []float32
	rotMatrix [][]float32
	enableRot bool
	trained   bool
}

func NewInt4Quantizer(dim int, enableRot bool) *Int4Quantizer {
	return &Int4Quantizer{
		dim:       dim,
		enableRot: enableRot,
	}
}

func (q *Int4Quantizer) Type() types.QuantizeType {
	return types.QuantizeTypeInt4
}

func (q *Int4Quantizer) CodeSize() int {
	return (q.dim + 1) / 2
}

func (q *Int4Quantizer) Train(vectors [][]float32) {
	n := len(vectors)
	if n == 0 {
		return
	}

	if q.enableRot {
		q.rotMatrix = computeRandomRotation(q.dim, 42)
	}

	q.scale = make([]float32, q.dim)
	for j := 0; j < q.dim; j++ {
		var maxAbs float32
		for i := 0; i < n; i++ {
			v := vectors[i]
			if j < len(v) {
				abs := float32(math.Abs(float64(v[j])))
				if abs > maxAbs {
					maxAbs = abs
				}
			}
		}
		if maxAbs > 0 {
			q.scale[j] = maxAbs
		} else {
			q.scale[j] = 1.0
		}
	}
	q.trained = true
}

func (q *Int4Quantizer) Encode(vec []float32, dst []byte) []byte {
	needed := (q.dim + 1) / 2
	if cap(dst) < needed {
		dst = make([]byte, needed)
	} else {
		dst = dst[:needed]
	}

	v := vec
	if q.enableRot && q.rotMatrix != nil {
		v = applyRotation(vec, q.rotMatrix)
	}

	for j := 0; j < q.dim; j++ {
		var val float32
		if j < len(v) {
			var s float32
			if j < len(q.scale) && q.scale[j] != 0 {
				s = q.scale[j]
			} else {
				s = 1.0
			}
			scaled := v[j] / s
			if scaled > 1.0 {
				scaled = 1.0
			} else if scaled < -1.0 {
				scaled = -1.0
			}
			val = scaled
		}
		nibble := uint8(int8(val*7.0)) & 0x0F
		if j%2 == 0 {
			dst[j/2] = nibble << 4
		} else {
			dst[j/2] |= nibble
		}
	}
	return dst
}

func (q *Int4Quantizer) Decode(src []byte, dst []float32) []float32 {
	n := q.dim
	if cap(dst) < n {
		dst = make([]float32, n)
	} else {
		dst = dst[:n]
	}
	for j := 0; j < n; j++ {
		var nibble uint8
		if j%2 == 0 {
			nibble = src[j/2] >> 4
		} else {
			nibble = src[j/2] & 0x0F
		}
		if nibble&0x08 != 0 {
			nibble |= 0xF0
		}
		dst[j] = float32(int8(nibble)) / 7.0
		if j < len(q.scale) {
			dst[j] *= q.scale[j]
		}
	}
	if q.enableRot && q.rotMatrix != nil {
		dst = applyInverseRotation(dst, q.rotMatrix)
	}
	return dst
}

type RaBitQQuantizer struct {
	dimension    int
	enableRotate bool
	rotation     [][]float32
	rng          *rand.Rand
}

func NewRaBitQQuantizer(dimension int, enableRotate bool) *RaBitQQuantizer {
	return &RaBitQQuantizer{
		dimension:    dimension,
		enableRotate: enableRotate,
		rng:          rand.New(rand.NewSource(42)),
	}
}

func (q *RaBitQQuantizer) Type() types.QuantizeType { return types.QuantizeTypeRaBitQ }

func (q *RaBitQQuantizer) Train(vectors [][]float32) {
	if q.enableRotate {
		q.rotation = computeRandomRotation(q.dimension, q.rng.Int63())
	}
}

func (q *RaBitQQuantizer) Encode(vec []float32, dst []byte) []byte {
	rotated := vec
	if q.enableRotate && q.rotation != nil {
		rotated = applyRotation(vec, q.rotation)
	}

	codeSize := (len(rotated) + 7) / 8
	if dst == nil {
		dst = make([]byte, codeSize+4)
	} else if len(dst) < codeSize+4 {
		dst = make([]byte, codeSize+4)
	} else {
		for i := range dst[:codeSize+4] {
			dst[i] = 0
		}
	}

	for i := 0; i < len(rotated); i++ {
		if rotated[i] >= 0 {
			dst[i/8] |= 1 << (i % 8)
		}
	}

	norm := float32(0)
	for _, v := range rotated {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	binary.LittleEndian.PutUint32(dst[codeSize:], math.Float32bits(norm))

	return dst
}

func (q *RaBitQQuantizer) Decode(src []byte, dst []float32) []float32 {
	codeSize := (q.dimension + 7) / 8
	if len(src) < codeSize+4 {
		return dst
	}

	if dst == nil {
		dst = make([]float32, q.dimension)
	}

	for i := 0; i < q.dimension; i++ {
		if src[i/8]&(1<<(i%8)) != 0 {
			dst[i] = 1.0
		} else {
			dst[i] = -1.0
		}
	}

	return dst
}

func (q *RaBitQQuantizer) CodeSize() int {
	return (q.dimension+7)/8 + 4
}

func float32ToFloat16(f float32) uint16 {
	bits := math.Float32bits(f)
	sign := uint16((bits >> 16) & 0x8000)
	exp := int((bits >> 23) & 0xFF)
	frac := bits & 0x7FFFFF

	var outExp, outFrac uint16
	switch {
	case exp == 0:
		outExp = 0
		outFrac = uint16(frac >> 13)
	case exp == 0xFF:
		outExp = 0x1F
		outFrac = uint16(frac >> 13)
	default:
		newexp := exp - 127 + 15
		if newexp >= 0x1F {
			outExp = 0x1F
			outFrac = 0
		} else if newexp <= 0 {
			outExp = 0
			outFrac = uint16((frac | 0x800000) >> (13 - newexp + 1))
		} else {
			outExp = uint16(newexp)
			outFrac = uint16(frac >> 13)
		}
	}
	return sign | (outExp << 10) | outFrac
}

func float16ToFloat32(h uint16) float32 {
	sign := uint32((h >> 15) & 0x1)
	exp := uint32((h >> 10) & 0x1F)
	frac := uint32(h & 0x3FF)

	var outExp, outFrac uint32
	switch {
	case exp == 0:
		if frac == 0 {
			outExp = 0
			outFrac = 0
		} else {
			shift := uint32(0)
			for (frac & 0x400) == 0 {
				frac <<= 1
				shift++
			}
			frac &^= 0x400
			outExp = 1 + (127 - 15) - shift
			outFrac = frac << 13
		}
	case exp == 0x1F:
		outExp = 0xFF
		outFrac = frac << 13
	default:
		outExp = exp + (127 - 15)
		outFrac = frac << 13
	}
	return math.Float32frombits((sign << 31) | (outExp << 23) | outFrac)
}

func computeRandomRotation(dim int, seed int64) [][]float32 {
	rng := rand.New(rand.NewSource(seed))
	mat := make([][]float32, dim)
	for i := 0; i < dim; i++ {
		mat[i] = make([]float32, dim)
		for j := 0; j < dim; j++ {
			mat[i][j] = float32(rng.NormFloat64())
		}
		norm := float32(0)
		for j := 0; j < dim; j++ {
			norm += mat[i][j] * mat[i][j]
		}
		norm = float32(math.Sqrt(float64(norm)))
		if norm > 0 {
			for j := 0; j < dim; j++ {
				mat[i][j] /= norm
			}
		}
	}
	return orthogonalize(mat, dim)
}

func orthogonalize(mat [][]float32, dim int) [][]float32 {
	for i := 0; i < dim; i++ {
		for k := 0; k < i; k++ {
			dot := float32(0)
			for j := 0; j < dim; j++ {
				dot += mat[i][j] * mat[k][j]
			}
			for j := 0; j < dim; j++ {
				mat[i][j] -= dot * mat[k][j]
			}
		}
		norm := float32(0)
		for j := 0; j < dim; j++ {
			norm += mat[i][j] * mat[i][j]
		}
		norm = float32(math.Sqrt(float64(norm)))
		if norm > 0 {
			for j := 0; j < dim; j++ {
				mat[i][j] /= norm
			}
		}
	}
	return mat
}

func applyRotation(vec []float32, rot [][]float32) []float32 {
	dim := len(rot)
	result := make([]float32, dim)
	for i := 0; i < dim; i++ {
		var sum float32
		for j := 0; j < len(vec) && j < dim; j++ {
			sum += rot[i][j] * vec[j]
		}
		result[i] = sum
	}
	return result
}

func applyInverseRotation(vec []float32, rot [][]float32) []float32 {
	dim := len(rot)
	result := make([]float32, dim)
	for j := 0; j < dim; j++ {
		var sum float32
		for i := 0; i < dim; i++ {
			sum += rot[i][j] * vec[i]
		}
		result[j] = sum
	}
	return result
}

type PQQuantizer struct {
	dimension int
	numSub    int
	subDim    int
	codebooks [][][]float32
	k         int
	rng       *rand.Rand
}

func NewPQQuantizer(dimension, numSubquantizers int) *PQQuantizer {
	subDim := dimension / numSubquantizers
	if dimension%numSubquantizers != 0 {
		subDim++
	}
	return &PQQuantizer{
		dimension: dimension,
		numSub:    numSubquantizers,
		subDim:    subDim,
		k:         256,
		rng:       rand.New(rand.NewSource(42)),
	}
}

func (q *PQQuantizer) Type() types.QuantizeType { return types.QuantizeTypePQ }

func (q *PQQuantizer) Train(vectors [][]float32) {
	if len(vectors) == 0 {
		return
	}

	q.codebooks = make([][][]float32, q.numSub)

	for s := 0; s < q.numSub; s++ {
		start := s * q.subDim
		end := start + q.subDim
		if end > q.dimension {
			end = q.dimension
		}
		actualSubDim := end - start

		subVectors := make([][]float32, len(vectors))
		for i, v := range vectors {
			sv := make([]float32, actualSubDim)
			copy(sv, v[start:end])
			subVectors[i] = sv
		}

		k := q.k
		if len(subVectors) < k {
			k = len(subVectors)
		}

		centroids := q.kMeans(subVectors, k, actualSubDim)
		q.codebooks[s] = centroids
	}
}

func (q *PQQuantizer) kMeans(data [][]float32, k, dim int) [][]float32 {
	centroids := make([][]float32, k)
	for i := 0; i < k; i++ {
		centroids[i] = make([]float32, dim)
		if i < len(data) {
			copy(centroids[i], data[i])
		}
	}

	assignments := make([]int, len(data))

	for iter := 0; iter < 20; iter++ {
		for i, v := range data {
			bestIdx := 0
			bestDist := float32(1e30)
			for j, c := range centroids {
				d := float32(0)
				for d2 := 0; d2 < dim; d2++ {
					diff := v[d2] - c[d2]
					d += diff * diff
				}
				if d < bestDist {
					bestDist = d
					bestIdx = j
				}
			}
			assignments[i] = bestIdx
		}

		counts := make([]int, k)
		sums := make([][]float32, k)
		for j := 0; j < k; j++ {
			sums[j] = make([]float32, dim)
		}

		for i, v := range data {
			a := assignments[i]
			counts[a]++
			for d := 0; d < dim; d++ {
				sums[a][d] += v[d]
			}
		}

		for j := 0; j < k; j++ {
			if counts[j] > 0 {
				for d := 0; d < dim; d++ {
					centroids[j][d] = sums[j][d] / float32(counts[j])
				}
			}
		}
	}

	return centroids
}

func (q *PQQuantizer) Encode(vec []float32, dst []byte) []byte {
	if q.codebooks == nil {
		return dst
	}

	if dst == nil {
		dst = make([]byte, q.numSub)
	} else if len(dst) < q.numSub {
		dst = make([]byte, q.numSub)
	}

	for s := 0; s < q.numSub; s++ {
		start := s * q.subDim
		end := start + q.subDim
		if end > q.dimension {
			end = q.dimension
		}
		actualSubDim := end - start

		bestCode := 0
		bestDist := float32(1e30)

		for code, centroid := range q.codebooks[s] {
			d := float32(0)
			for d2 := 0; d2 < actualSubDim && d2 < len(centroid); d2++ {
				diff := vec[start+d2] - centroid[d2]
				d += diff * diff
			}
			if d < bestDist {
				bestDist = d
				bestCode = code
			}
		}

		dst[s] = byte(bestCode)
	}

	return dst
}

func (q *PQQuantizer) Decode(src []byte, dst []float32) []float32 {
	if q.codebooks == nil || len(src) < q.numSub {
		return dst
	}

	if dst == nil {
		dst = make([]float32, q.dimension)
	}

	for s := 0; s < q.numSub; s++ {
		start := s * q.subDim
		code := int(src[s])
		if code >= len(q.codebooks[s]) {
			continue
		}
		centroid := q.codebooks[s][code]
		for d := 0; d < len(centroid) && start+d < q.dimension; d++ {
			dst[start+d] = centroid[d]
		}
	}

	return dst
}

func (q *PQQuantizer) CodeSize() int {
	return q.numSub
}

func (q *PQQuantizer) ApproximateDistance(code []byte, query []float32) float32 {
	if q.codebooks == nil || len(code) < q.numSub {
		return 0
	}

	var dist float32
	for s := 0; s < q.numSub; s++ {
		start := s * q.subDim
		end := start + q.subDim
		if end > q.dimension {
			end = q.dimension
		}

		c := int(code[s])
		if c >= len(q.codebooks[s]) {
			continue
		}
		centroid := q.codebooks[s][c]
		for d := 0; d < len(centroid) && start+d < len(query); d++ {
			diff := query[start+d] - centroid[d]
			dist += diff * diff
		}
	}
	return dist
}

func quantizeByType(vec []float32, qt types.QuantizeType, dim int, enableRot bool) ([]byte, error) {
	switch qt {
	case types.QuantizeTypeFP16:
		q := NewFP16Quantizer()
		return q.Encode(vec, nil), nil
	case types.QuantizeTypeInt8:
		q := NewInt8Quantizer(dim, enableRot)
		q.Train([][]float32{vec})
		return q.Encode(vec, nil), nil
	case types.QuantizeTypeInt4:
		q := NewInt4Quantizer(dim, enableRot)
		q.Train([][]float32{vec})
		return q.Encode(vec, nil), nil
	case types.QuantizeTypeRaBitQ:
		q := NewRaBitQQuantizer(dim, enableRot)
		q.Train([][]float32{vec})
		return q.Encode(vec, nil), nil
	default:
		return nil, fmt.Errorf("unsupported quantize type: %v", qt)
	}
}
