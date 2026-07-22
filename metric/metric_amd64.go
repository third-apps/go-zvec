package metric

import "golang.org/x/sys/cpu"

func init() {
	if cpu.X86.HasAVX2 && cpu.X86.HasFMA {
		l2Impl = l2AVX2
		ipImpl = ipAVX2
	}
}
