package types

type MetricType uint32

const (
	MetricTypeUndefined MetricType = 0
	MetricTypeL2        MetricType = 1
	MetricTypeIP        MetricType = 2
	MetricTypeCosine    MetricType = 3
	MetricTypeMIPSL2    MetricType = 4
)

func (m MetricType) String() string {
	switch m {
	case MetricTypeUndefined:
		return "undefined"
	case MetricTypeL2:
		return "l2"
	case MetricTypeIP:
		return "ip"
	case MetricTypeCosine:
		return "cosine"
	case MetricTypeMIPSL2:
		return "mipsl2"
	default:
		return "unknown"
	}
}
