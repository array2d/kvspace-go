// Package shm provides kvspace-c SHM backend via cgo.
package shm

/*
#cgo LDFLAGS: -L${SRCDIR}/../../kvspace-c/build -lkvspace-c -lpthread -lrt
#cgo CFLAGS: -I${SRCDIR}/../../kvspace-c/include
#include <stdlib.h>
#include <unistd.h>
#include "kvspace/kvspace.h"
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"

	"github.com/array2d/kvspace-go"
)

func init() { kvspace.Register("shm", Conn) }

type store struct{ ptr *C.kvspace_t }

func Conn(addr string) kvspace.KVSpace {
	path := addr
	if path == "" {
		path = "/tmp/kvspace_shm_default"
	}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	ptr := C.kvspace_open(cpath, C.size_t(8*64*64*64)) // 2MB
	if ptr == nil {
		panic(fmt.Sprintf("shm: cannot open %s", path))
	}
	return &store{ptr: ptr}
}

func (s *store) Get(prefix string, keys []string, resolve bool) []kvspace.XValue {
	results := make([]kvspace.XValue, len(keys))
	for i, key := range keys {
		full := kvspace.JoinPath(prefix, key)
		ck := C.CString(full)
		var vl C.int32_t
		var cResolve C.int
		if resolve {
			cResolve = 1
		}
		v := C.kvspace_get(s.ptr, ck, cResolve, &vl)
		C.free(unsafe.Pointer(ck))
		if v == nil || vl == 0 {
			results[i] = kvspace.None{}
			continue
		}
		buf := C.GoBytes(unsafe.Pointer(v), vl)
		results[i] = kvspace.DecodeXValue(buf)
	}
	return results
}

func (s *store) Set(pairs []kvspace.KVPair) error {
	for _, p := range pairs {
		if ptr, ok := p.Val.(kvspace.Ptr); ok {
			if err := kvspace.ValidatePtr(s, ptr.Target(), ptr.Kind(), ptr.ArrayLen()); err != nil {
				return err
			}
		}
		ck := C.CString(p.Key)
		encoded := p.Val.Encode()
		cv := C.CBytes(encoded)
		r := C.kvspace_set(s.ptr, ck, (*C.uint8_t)(cv), C.int32_t(len(encoded)))
		C.free(cv)
		C.free(unsafe.Pointer(ck))
		if r != 0 {
			return fmt.Errorf("shm: set %s failed", p.Key)
		}
	}
	return nil
}

func (s *store) List(prefix string, expandExt bool, resolve bool) []string {
	ck := C.CString(prefix)
	var names **C.char
	var count C.int32_t
	var cResolve C.int
	if resolve {
		cResolve = 1
	}
	C.kvspace_list(s.ptr, ck, C.bool(expandExt), cResolve, &names, &count)
	C.free(unsafe.Pointer(ck))

	result := make([]string, 0, int(count))
	for i := 0; i < int(count); i++ {
		ptr := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(names)) + uintptr(i)*unsafe.Sizeof(*names)))
		result = append(result, C.GoString(*ptr))
		C.free(unsafe.Pointer(*ptr))
	}
	if names != nil {
		C.free(unsafe.Pointer(names))
	}
	return result
}

func (s *store) Del(keys ...string) error {
	for _, k := range keys {
		ck := C.CString(k)
		C.kvspace_del(s.ptr, ck)
		C.free(unsafe.Pointer(ck))
	}
	return nil
}

func (s *store) DelTree(prefix string) error {
	ck := C.CString(prefix)
	C.kvspace_deltree(s.ptr, ck)
	C.free(unsafe.Pointer(ck))
	return nil
}

func (s *store) Mkindex(path string) error {
	ck := C.CString(path)
	C.kvspace_mkindex(s.ptr, ck)
	C.free(unsafe.Pointer(ck))
	return nil
}

func (s *store) ExtIndex(path, extpath string) error {
	cp := C.CString(path)
	ce := C.CString(extpath)
	r := C.kvspace_extindex(s.ptr, cp, ce)
	C.free(unsafe.Pointer(cp))
	C.free(unsafe.Pointer(ce))
	if r != 0 {
		return fmt.Errorf("shm: extindex %s -> %s failed", path, extpath)
	}
	return nil
}

func (s *store) DelExtIndex(path string) error {
	ck := C.CString(path)
	C.kvspace_del(s.ptr, ck)
	C.free(unsafe.Pointer(ck))
	return nil
}

func (s *store) Notify(key string, val kvspace.XValue) error {
	ck := C.CString(key)
	encoded := val.Encode()
	cv := C.CBytes(encoded)
	C.kvspace_notify(s.ptr, ck, (*C.uint8_t)(cv), C.int32_t(len(encoded)))
	C.free(cv)
	C.free(unsafe.Pointer(ck))
	return nil
}

func (s *store) Watch(key string, timeout time.Duration) kvspace.XValue {
	ck := C.CString(key)
	var vl C.int32_t
	v := C.kvspace_watch(s.ptr, ck, C.int32_t(timeout.Milliseconds()), &vl)
	C.free(unsafe.Pointer(ck))
	if v == nil || vl == 0 {
		return kvspace.None{}
	}
	buf := C.GoBytes(unsafe.Pointer(v), vl)
	return kvspace.DecodeXValue(buf)
}

func (s *store) Clear() error {
	C.kvspace_close(s.ptr)
	ck := C.CString("/tmp/kvspace_clear")
	C.unlink(ck)
	s.ptr = C.kvspace_open(ck, C.size_t(32768))
	C.free(unsafe.Pointer(ck))
	if s.ptr == nil {
		return fmt.Errorf("shm: clear failed")
	}
	return nil
}

func (s *store) DisConn() error {
	C.kvspace_close(s.ptr)
	return nil
}
