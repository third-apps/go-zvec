package types

type QuantizeType uint32

const (
	QuantizeTypeUndefined QuantizeType = 0
	QuantizeTypeFP16      QuantizeType = 1
	QuantizeTypeInt8      QuantizeType = 2
	QuantizeTypeInt4      QuantizeType = 3
	QuantizeTypeRaBitQ    QuantizeType = 4
	QuantizeTypePQ        QuantizeType = 5
)

func (q QuantizeType) String() string {
	switch q {
	case QuantizeTypeUndefined:
		return "undefined"
	case QuantizeTypeFP16:
		return "fp16"
	case QuantizeTypeInt8:
		return "int8"
	case QuantizeTypeInt4:
		return "int4"
	case QuantizeTypeRaBitQ:
		return "rabitq"
	case QuantizeTypePQ:
		return "pq"
	default:
		return "unknown"
	}
}
