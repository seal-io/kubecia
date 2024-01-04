package bytespool

import (
	"bytes"
	"sync"
)

const defaultBytesSize = 32 * 1024

var (
	bytesPool = sync.Pool{
		New: func() any {
			b := make([]byte, defaultBytesSize)
			return &b
		},
	}

	bufferPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(GetBytes(0))
		},
	}
)

func GetBytes(length int) []byte {
	var (
		bsp = bytesPool.Get().(*[]byte)
		bs  = *bsp
	)

	if length <= 0 {
		length = defaultBytesSize
	}

	if cap(bs) >= length {
		return bs[:length]
	}

	bytesPool.Put(bsp)

	return make([]byte, length)
}

func GetBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

func Put(b any) {
	switch t := b.(type) {
	case *[]byte:
		bytesPool.Put(t)
	case []byte:
		bytesPool.Put(&t)
	case *bytes.Buffer:
		t.Reset()
		bufferPool.Put(t)
	case bytes.Buffer:
		t.Reset()
		bufferPool.Put(&t)
	}
}
