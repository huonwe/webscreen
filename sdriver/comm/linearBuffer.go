package comm

// LinearBuffer 管理器
type LinearBuffer struct {
	buf    []byte
	offset int
	size   int
}

func NewLinearBuffer(size int) *LinearBuffer {
	if size == 0 {
		size = 20 * 1024 * 1024 // 默认 20MB
	}
	return &LinearBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

// Get 获取一段空闲内存用于写入。如果空间不足，返回 nil
func (lb *LinearBuffer) Get(length int) []byte {
	if lb.offset+length > lb.size {
		start := 0
		lb.offset = length
		return lb.buf[start:lb.offset]
	}
	start := lb.offset
	lb.offset += length
	return lb.buf[start:lb.offset]
}
