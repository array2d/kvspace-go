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
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/array2d/kvspace-go"
)

func init() { kvspace.Register("shm", Conn) }

type store struct {
	mu   sync.RWMutex
	ptr  *C.kvspace_t
	exts map[string]string
	done chan struct{}
}

func Conn(addr string) kvspace.KVSpace {
	path := addr
	if path == "" {
		path = "/tmp/kvspace_shm_default"
	}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	// open existing or create new; kvspace_open handles both
	ptr := C.kvspace_open(cpath, C.size_t(8*64*64*64)) // 2MB
	if ptr == nil {
		panic(fmt.Sprintf("shm: cannot open %s", path))
	}
	return &store{ptr: ptr, exts: make(map[string]string), done: make(chan struct{})}
}

// -- helpers --
func (s *store) getRaw(path string, resolve bool) (kvspace.XValue, bool) {
	ck := C.CString(path)
	defer C.free(unsafe.Pointer(ck))
	var vl C.int32_t
	var cResolve C.int
	if resolve { cResolve = 1 }
	v := C.kvspace_get(s.ptr, ck, cResolve, &vl)
	if v == nil || vl == 0 {
		return kvspace.None{}, false
	}
	// v is direct SHM pointer (zero-copy), do NOT free
	buf := C.GoBytes(unsafe.Pointer(v), vl)
	return kvspace.DecodeXValueHead(buf).Decode(), true
}

func (s *store) setRaw(path string, val kvspace.XValue) error {
	ck := C.CString(path); defer C.free(unsafe.Pointer(ck))
	encoded := val.Encode()
	cv := C.CBytes(encoded)
	r := C.kvspace_set(s.ptr, ck, (*C.uint8_t)(cv), C.int32_t(len(encoded)))
	C.free(cv)
	if r != 0 {
		return fmt.Errorf("shm: set %s failed (ret=%d len=%d ptr=%p)", path, int(r), len(encoded), s.ptr)
	}
	return nil
}

// -- KVSpace interface --

func (s *store) Get(prefix string, keys []string, resolve bool) []kvspace.XValue {
	s.mu.RLock(); defer s.mu.RUnlock()
	results := make([]kvspace.XValue, len(keys))
	for i, key := range keys {
		full := kvspace.JoinPath(prefix, key)
		if v, ok := s.getRaw(full, resolve); ok {
			results[i] = v
		} else {
			results[i] = kvspace.None{}
		}
	}
	return results
}

func (s *store) Set(pairs []kvspace.KVPair) error {
	s.mu.Lock(); defer s.mu.Unlock()
	for _, p := range pairs {
		// auto-create parent directories
		parent, _ := kvspace.SepPath(p.Key)
		if parent != "" && parent != "/" {
			s.mkindexLocked(parent + "/")
		}
		if err := s.setRaw(p.Key, p.Val); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) mkindexLocked(path string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 1; i <= len(parts); i++ {
		dir := "/" + strings.Join(parts[:i], "/") + "/"
		if _, ok := s.getRaw(dir, false); !ok {
			ck := C.CString(dir); C.kvspace_mkindex(s.ptr, ck); C.free(unsafe.Pointer(ck))
		}
	}
}

func (s *store) List(prefix string, expandExt bool, resolve bool) []string {
	s.mu.RLock(); defer s.mu.RUnlock()
	ck := C.CString(prefix); defer C.free(unsafe.Pointer(ck))
	var names **C.char
	var count C.int32_t
	var cResolve C.int
	if resolve { cResolve = 1 }
	C.kvspace_list(s.ptr, ck, C.bool(expandExt), cResolve, &names, &count)
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
	s.mu.Lock(); defer s.mu.Unlock()
	for _, k := range keys {
		ck := C.CString(k); C.kvspace_del(s.ptr, ck); C.free(unsafe.Pointer(ck))
	}
	return nil
}

func (s *store) DelTree(prefix string) error {
	s.mu.Lock(); defer s.mu.Unlock()
	ck := C.CString(prefix); defer C.free(unsafe.Pointer(ck))
	C.kvspace_deltree(s.ptr, ck)
	return nil
}

func (s *store) Mkindex(path string) error {
	s.mu.Lock(); defer s.mu.Unlock()
	// Recursively create parent directories
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 1; i <= len(parts); i++ {
		dir := "/" + strings.Join(parts[:i], "/") + "/"
		ck := C.CString(dir); C.kvspace_mkindex(s.ptr, ck); C.free(unsafe.Pointer(ck))
	}
	return nil
}

func (s *store) Link(target, linkpath string) error {
	s.mu.Lock(); defer s.mu.Unlock()
	ct := C.CString(target); defer C.free(unsafe.Pointer(ct))
	cl := C.CString(linkpath); defer C.free(unsafe.Pointer(cl))
	if C.kvspace_link(s.ptr, ct, cl) != 0 {
		return fmt.Errorf("shm: link %s -> %s failed", linkpath, target)
	}
	return nil
}

func (s *store) ExtIndex(path, extpath string) error {
	s.mu.Lock(); defer s.mu.Unlock()
	s.exts[path] = extpath
	cp := C.CString(path); defer C.free(unsafe.Pointer(cp))
	ce := C.CString(extpath); defer C.free(unsafe.Pointer(ce))
	if C.kvspace_extindex(s.ptr, cp, ce) != 0 {
		return fmt.Errorf("shm: extindex %s -> %s failed", path, extpath)
	}
	return nil
}

func (s *store) DelExtIndex(path string) error {
	s.mu.Lock(); defer s.mu.Unlock()
	delete(s.exts, path)
	ck := C.CString(path); defer C.free(unsafe.Pointer(ck))
	C.kvspace_unlink(s.ptr, ck)
	return nil
}

func (s *store) Notify(key string, val kvspace.XValue) error {
	s.mu.Lock(); defer s.mu.Unlock()
	ck := C.CString(key); defer C.free(unsafe.Pointer(ck))
	encoded := val.Encode()
	cv := C.CBytes(encoded); defer C.free(unsafe.Pointer(cv))
	C.kvspace_notify(s.ptr, ck, (*C.uint8_t)(cv), C.int32_t(len(encoded)))
	return nil
}

func (s *store) Watch(key string, timeout time.Duration) kvspace.XValue {
	ck := C.CString(key); defer C.free(unsafe.Pointer(ck))
	var vl C.int32_t
	v := C.kvspace_watch(s.ptr, ck, C.int32_t(timeout.Milliseconds()), &vl)
	if v == nil || vl == 0 {
		return kvspace.None{}
	}
	buf := C.GoBytes(unsafe.Pointer(v), vl)
	return kvspace.DecodeXValueHead(buf).Decode()
}

func (s *store) Clear() error {
	s.mu.Lock(); defer s.mu.Unlock()
	s.exts = make(map[string]string)
	// reset: close and reopen
	C.kvspace_close(s.ptr)
	ck := C.CString("/tmp/kvspace_clear"); C.unlink(ck)
	s.ptr = C.kvspace_open(ck, C.size_t(32768))
	C.free(unsafe.Pointer(ck))
	if s.ptr == nil {
		return fmt.Errorf("shm: clear failed")
	}
	return nil
}

func (s *store) DisConn() error {
	C.kvspace_close(s.ptr)
	close(s.done)
	return nil
}
