// Package goheap 提供进程内 ART 存储。
package goheap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/array2d/kvspace-go"
)

const maxLinkResolutions = 64

func init() { kvspace.Register("goheap", Conn) }

var (
	namedStoresMu sync.Mutex
	namedStores   = map[string]*store{}
)

func Conn(addr string) kvspace.KVSpace {
	if addr == "" {
		addr = "local"
	}
	namedStoresMu.Lock()
	s := namedStores[addr]
	if s == nil {
		s = &store{
			exts:   make(map[string]string),
			queues: make(map[string]*notifyQueue),
		}
		namedStores[addr] = s
	}
	s.refs++
	namedStoresMu.Unlock()
	return &backend{store: s, done: make(chan struct{})}
}

func Destroy(addr string) error {
	if addr == "" {
		addr = "local"
	}
	namedStoresMu.Lock()
	defer namedStoresMu.Unlock()
	s := namedStores[addr]
	if s == nil {
		return nil
	}
	if s.refs != 0 {
		return fmt.Errorf("art: store %q has %d active connection(s)", addr, s.refs)
	}
	delete(namedStores, addr)
	return nil
}

type store struct {
	mu   sync.RWMutex
	tree radixTree
	exts map[string]string
	refs int

	queuesMu sync.Mutex
	queues   map[string]*notifyQueue
}

type backend struct {
	store     *store
	lifeMu    sync.RWMutex
	closeOnce sync.Once
	done      chan struct{}
}

var _ kvspace.KVSpace = (*backend)(nil)

func (b *backend) Get(prefix string, keys []string, resolve bool) []kvspace.XValue {
	b.mustBegin()
	defer b.end()
	mustValidDirPath(prefix)
	for _, key := range keys {
		if err := validateChildKey(key); err != nil {
			panic(err)
		}
	}
	b.store.mu.RLock()
	defer b.store.mu.RUnlock()

	if resolve {
		prefix = b.resolvePathLocked(prefix)
	}
	extPrefix, hasExt := b.extFallbackPathLocked(prefix)

	results := make([]kvspace.XValue, len(keys))
	for i, key := range keys {
		full := kvspace.JoinPath(prefix, key)
		if resolve {
			full = b.resolvePathLocked(full)
		}
		if v, ok := b.valueLocked(full); ok {
			results[i] = v
			continue
		}
		if hasExt {
			target := b.resolvePathLocked(kvspace.JoinPath(extPrefix, key))
			if v, ok := b.valueLocked(target); ok {
				results[i] = v
				continue
			}
		}
		results[i] = kvspace.None{}
	}
	return results
}

func (b *backend) Set(pairs []kvspace.KVPair) error {
	if err := b.begin(); err != nil {
		return err
	}
	defer b.end()

	type preparedPair struct {
		key      string
		encoded  []byte
		children []string
		dir      bool
	}
	prepared := make([]preparedPair, 0, len(pairs))
	var validationErr error
	for _, pair := range pairs {
		if err := validateAbsolutePath(pair.Key); err != nil {
			validationErr = err
			break
		}
		encoded, stored, err := encodeStoredValue(pair.Val)
		if err != nil {
			validationErr = fmt.Errorf("%w: %s", err, pair.Key)
			break
		}
		var children []string
		dir := isDir(pair.Key)
		if dir {
			if stored.Kind() != kvspace.KindIndex {
				validationErr = fmt.Errorf("%w: %s has kind=%s", kvspace.ErrInvalidDirValue, pair.Key, stored.Kind())
				break
			}
			children = normalizeIndex(indexChildren(stored))
			for _, child := range children {
				if err := validateIndexChild(child); err != nil {
					validationErr = fmt.Errorf("%w: %s", err, pair.Key)
					break
				}
			}
			if validationErr != nil {
				break
			}
		} else {
			switch stored.Kind() {
			case kvspace.KindIndex, kvspace.KindLinkIndex, kvspace.KindExtIndex:
				validationErr = fmt.Errorf("%w: %s has reserved kind=%s", kvspace.ErrInvalidValue, pair.Key, stored.Kind())
			}
			if validationErr != nil {
				break
			}
		}
		prepared = append(prepared, preparedPair{key: pair.Key, children: children, dir: dir})
		if !dir {
			prepared[len(prepared)-1].encoded = cloneBytes(encoded)
		}
	}
	if len(prepared) == 0 {
		return validationErr
	}

	applyErr := func() error {
		b.store.mu.Lock()
		defer b.store.mu.Unlock()
		indexes := make(indexCache)
		defer b.flushIndexesLocked(indexes)
		for _, pair := range prepared {
			resolved := b.resolvePathLocked(pair.key)
			if err := b.checkExtTargetMutationLocked(resolved, indexes, pair.children, pair.dir); err != nil {
				return err
			}
			parent, name := parentName(resolved)
			if err := b.ensureDirCachedLocked(parent, indexes); err != nil {
				return err
			}
			if err := b.checkExtMutationLocked(resolved, kvspace.ErrExtWrite, indexes); err != nil {
				return err
			}
			if pair.dir {
				if err := b.replaceDirectoryCachedLocked(resolved, pair.children, indexes); err != nil {
					return err
				}
			} else {
				b.store.tree.Insert([]byte(resolved), pair.encoded)
			}
			b.addChildCachedLocked(parent, indexChild(resolved, name), indexes)
		}
		return nil
	}()
	if applyErr != nil {
		return applyErr
	}
	return validationErr
}

func (b *backend) List(prefix string, expandExt bool, resolve bool) []string {
	b.mustBegin()
	defer b.end()
	mustValidDirPath(prefix)
	b.store.mu.RLock()
	defer b.store.mu.RUnlock()

	resolved := prefix
	if resolve {
		resolved = b.resolvePathLocked(prefix)
	}
	local := b.readDirIndexLocked(resolved)
	if !expandExt {
		return cloneStrings(local)
	}
	extTarget, ok := b.extFallbackPathLocked(resolved)
	if !ok {
		return cloneStrings(local)
	}
	extended := b.readDirIndexLocked(extTarget)
	result := make([]string, 0, len(local)+len(extended))
	result = append(result, local...)
	seen := make(map[string]struct{}, len(local))
	for _, name := range local {
		seen[name] = struct{}{}
	}
	for _, name := range extended {
		if _, exists := seen[name]; exists {
			continue
		}
		result = append(result, name)
	}
	return result
}

func (b *backend) Del(keys ...string) error {
	if err := b.begin(); err != nil {
		return err
	}
	defer b.end()
	b.store.mu.Lock()
	defer b.store.mu.Unlock()
	for _, key := range keys {
		if err := validateAbsolutePath(key); err != nil {
			return err
		}
		if err := b.delLocked(key); err != nil {
			return err
		}
	}
	return nil
}

func (b *backend) DelTree(prefix string) error {
	if err := b.begin(); err != nil {
		return err
	}
	defer b.end()
	if err := validateAbsolutePath(prefix); err != nil {
		return err
	}
	b.store.mu.Lock()
	defer b.store.mu.Unlock()

	finalPath := b.resolveParentLocked(prefix)
	if v, ok := b.valueLocked(finalPath); ok && v.Kind() == kvspace.KindLinkIndex {
		return b.delLocked(prefix)
	}

	resolved := b.resolvePathLocked(prefix)
	parent, name := parentName(resolved)
	if err := b.checkExtMutationLocked(resolved, kvspace.ErrExtDel, nil); err != nil {
		return err
	}

	doomed := b.collectTreeKeysLocked(resolved)
	protectExts := len(b.store.exts) != 0
	var savedValues map[string][]byte
	var savedExts map[string]string
	if protectExts {
		savedValues = make(map[string][]byte, len(doomed))
		savedExts = make(map[string]string)
	}
	validate := false
	parents := make(map[string]map[string]struct{})
	for _, key := range doomed {
		path := string(key)
		if protectExts {
			data, _ := b.store.tree.Get(key)
			savedValues[path] = data
			kind, _, _ := encodedValueParts(data)
			validate = validate || bytes.Equal(kind, []byte(kvspace.KindIndex)) ||
				bytes.Equal(kind, []byte(kvspace.KindLinkIndex))
			if extPath, ok := b.store.exts[path]; ok {
				savedExts[path] = extPath
			}
		}
		parent, child := parentName(path)
		child = indexChild(path, child)
		if child != "" {
			if parents[parent] == nil {
				parents[parent] = make(map[string]struct{})
			}
			parents[parent][child] = struct{}{}
		}
		b.store.tree.Delete(key)
		delete(b.store.exts, path)
	}
	if validate && len(b.store.exts) != 0 {
		if err := b.validateExtTargetsLocked(); err != nil {
			for path, data := range savedValues {
				b.store.tree.Insert([]byte(path), data)
			}
			for path, extPath := range savedExts {
				b.store.exts[path] = extPath
			}
			return err
		}
	}
	for candidateParent, children := range parents {
		for child := range children {
			if !b.entryExistsLocked(candidateParent, child) {
				b.removeChildLocked(candidateParent, child)
			}
		}
	}
	name = indexChild(resolved, name)
	if len(doomed) == 0 && !b.entryExistsLocked(parent, name) {
		b.removeChildLocked(parent, name)
	}
	return nil
}

func (b *backend) Notify(key string, val kvspace.XValue) error {
	if err := b.begin(); err != nil {
		return err
	}
	defer b.end()
	if err := validateAbsolutePath(key); err != nil {
		return err
	}
	encoded, _, err := encodeStoredValue(val)
	if err != nil {
		return fmt.Errorf("%w: %s", err, key)
	}
	b.store.mu.RLock()
	defer b.store.mu.RUnlock()
	resolved := b.resolvePathLocked(key)
	queue := b.acquireQueue(resolved)
	defer b.releaseQueue(resolved, queue)
	queue.push(encoded)
	return nil
}

func (b *backend) Watch(key string, timeout time.Duration) kvspace.XValue {
	b.mustBegin()
	defer b.end()
	mustValidPath(key)
	resolved, queue := b.resolvedQueue(key)
	data, ok := queue.pop(timeout, b.done)
	b.releaseQueue(resolved, queue)
	if !ok {
		return kvspace.None{}
	}
	return kvspace.DecodeXValueHead(data).Decode()
}

func (b *backend) Mkindex(path string) error {
	if err := b.begin(); err != nil {
		return err
	}
	defer b.end()
	if err := validateDirPath(path); err != nil {
		return err
	}
	b.store.mu.Lock()
	defer b.store.mu.Unlock()
	return b.ensureDirLocked(b.resolvePathLocked(path))
}

func (b *backend) Link(target, linkpath string) error {
	if err := b.begin(); err != nil {
		return err
	}
	defer b.end()
	if err := validateAbsolutePath(target); err != nil {
		return err
	}
	if err := validateAbsolutePath(linkpath); err != nil {
		return err
	}
	if isDir(target) != isDir(linkpath) {
		return fmt.Errorf("%w: %s → %s", kvspace.ErrLinkTypeMismatch, target, linkpath)
	}

	b.store.mu.Lock()
	defer b.store.mu.Unlock()
	resolved := b.resolveParentLocked(linkpath)
	parent, name := parentName(resolved)
	if err := b.checkExtMutationLocked(resolved, kvspace.ErrExtWrite, nil); err != nil {
		return err
	}
	if value, ok := b.valueLocked(resolved); ok && value.Kind() != kvspace.KindLinkIndex {
		return fmt.Errorf("%w: %s", kvspace.ErrLinkPathExists, resolved)
	}
	oldValue, hadOldValue := b.store.tree.Get([]byte(resolved))
	b.putValueLocked(resolved, kvspace.NewLinkIndex(target))
	restore := func() {
		if hadOldValue {
			b.store.tree.Insert([]byte(resolved), oldValue)
		} else {
			b.store.tree.Delete([]byte(resolved))
		}
	}
	if _, err := b.resolvePathErrLocked(resolved); err != nil {
		restore()
		return err
	}
	if err := b.validateExtTargetsLocked(); err != nil {
		restore()
		return err
	}
	if err := b.canEnsureDirLocked(parent); err != nil {
		restore()
		return err
	}
	if err := b.ensureDirLocked(parent); err != nil {
		restore()
		return err
	}
	b.addChildLocked(parent, indexChild(resolved, name))
	return nil
}

func (b *backend) ExtIndex(path, extpath string) error {
	if err := b.begin(); err != nil {
		return err
	}
	defer b.end()
	if err := validateDirPath(path); err != nil {
		return err
	}
	if err := validateDirPath(extpath); err != nil {
		return err
	}

	b.store.mu.Lock()
	defer b.store.mu.Unlock()

	resolved := b.resolveParentLocked(path)
	parent, name := parentName(resolved)
	if err := b.checkExtMutationLocked(resolved, kvspace.ErrExtWrite, nil); err != nil {
		return err
	}

	var local []string
	if v, ok := b.valueLocked(resolved); ok {
		switch v.Kind() {
		case kvspace.KindIndex:
			local = normalizeIndex(indexChildren(v))
		case kvspace.KindExtIndex:
			local, _ = extIndexParts(v)
		default:
			return fmt.Errorf("%w: %s", kvspace.ErrNotDir, resolved)
		}
	}
	oldValue, hadOldValue := b.store.tree.Get([]byte(resolved))
	oldExtPath, hadOldExt := b.store.exts[resolved]
	b.putValueLocked(resolved, kvspace.NewExtIndex(local, extpath))
	b.store.exts[resolved] = extpath
	restore := func() {
		if hadOldValue {
			b.store.tree.Insert([]byte(resolved), oldValue)
		} else {
			b.store.tree.Delete([]byte(resolved))
		}
		if hadOldExt {
			b.store.exts[resolved] = oldExtPath
		} else {
			delete(b.store.exts, resolved)
		}
	}
	if err := b.validateExtTargetsLocked(); err != nil {
		restore()
		return err
	}
	if err := b.canEnsureDirLocked(parent); err != nil {
		restore()
		return err
	}
	if err := b.ensureDirLocked(parent); err != nil {
		restore()
		return err
	}
	b.addChildLocked(parent, indexChild(resolved, name))
	return nil
}

func (b *backend) UnLink(path string) error {
	if err := b.begin(); err != nil {
		return err
	}
	defer b.end()
	if err := validateAbsolutePath(path); err != nil {
		return err
	}
	b.store.mu.Lock()
	defer b.store.mu.Unlock()

	resolved := b.resolveParentLocked(path)
	if v, ok := b.valueLocked(resolved); ok && v.Kind() == kvspace.KindLinkIndex {
		return b.delResolvedLocked(resolved)
	}

	if v, ok := b.valueLocked(resolved); ok && v.Kind() == kvspace.KindExtIndex {
		children, _ := extIndexParts(v)
		b.putValueLocked(resolved, kvspace.NewIndex(children))
		delete(b.store.exts, resolved)
	}
	return nil
}

func (b *backend) Clear() error {
	if err := b.begin(); err != nil {
		return err
	}
	defer b.end()
	b.store.mu.Lock()
	defer b.store.mu.Unlock()
	b.store.tree = radixTree{}
	b.store.exts = make(map[string]string)

	b.store.queuesMu.Lock()
	for key, queue := range b.store.queues {
		queue.clear()
		if queue.idle() {
			delete(b.store.queues, key)
		}
	}
	b.store.queuesMu.Unlock()
	return nil
}

func (b *backend) DisConn() error {
	b.closeOnce.Do(func() {
		close(b.done)
		b.lifeMu.Lock()
		defer b.lifeMu.Unlock()
		namedStoresMu.Lock()
		b.store.refs--
		namedStoresMu.Unlock()
	})
	return nil
}

func (b *backend) delLocked(key string) error {
	resolved := b.resolveParentLocked(key)
	return b.delResolvedLocked(resolved)
}

func (b *backend) delResolvedLocked(resolved string) error {
	parent, name := parentName(resolved)
	name = indexChild(resolved, name)
	if err := b.checkExtMutationLocked(resolved, kvspace.ErrExtDel, nil); err != nil {
		return err
	}

	protectExts := len(b.store.exts) != 0
	var oldValue []byte
	var hadValue bool
	if protectExts {
		oldValue, hadValue = b.store.tree.Get([]byte(resolved))
	}
	b.store.tree.Delete([]byte(resolved))
	delete(b.store.exts, resolved)
	if protectExts && hadValue && len(b.store.exts) != 0 {
		kind, _, _ := encodedValueParts(oldValue)
		if bytes.Equal(kind, []byte(kvspace.KindIndex)) || bytes.Equal(kind, []byte(kvspace.KindLinkIndex)) {
			if err := b.validateExtTargetsLocked(); err != nil {
				b.store.tree.Insert([]byte(resolved), oldValue)
				return err
			}
		}
	}
	if !b.entryExistsLocked(parent, name) {
		b.removeChildLocked(parent, name)
	}
	return nil
}

func (b *backend) ensureDirLocked(path string) error {
	indexes := make(indexCache)
	defer b.flushIndexesLocked(indexes)
	return b.ensureDirCachedLocked(path, indexes)
}

func (b *backend) canEnsureDirLocked(path string) error {
	if !isDir(path) {
		return fmt.Errorf("%w: %s", kvspace.ErrDirMustEndWithSlash, path)
	}
	check := func(current, name string) error {
		if value, ok := b.valueLocked(current); ok {
			if value.Kind() != kvspace.KindIndex && value.Kind() != kvspace.KindExtIndex {
				return fmt.Errorf("%w: %s", kvspace.ErrNotDir, current)
			}
			return nil
		}
		if name != "" {
			if err := b.checkExtTargetMutationLocked(current, nil, nil, false); err != nil {
				return err
			}
			return b.checkExtMutationLocked(current, kvspace.ErrExtWrite, nil)
		}
		return nil
	}
	current := kvspace.PathSep
	if err := check(current, ""); err != nil {
		return err
	}
	if path == kvspace.PathSep {
		return nil
	}
	for _, name := range strings.Split(strings.Trim(path, kvspace.PathSep), kvspace.PathSep) {
		current = kvspace.JoinPath(current, name) + kvspace.DirIndexSuf
		if err := check(current, name); err != nil {
			return err
		}
	}
	return nil
}

func (b *backend) ensureDirCachedLocked(path string, indexes indexCache) error {
	if !isDir(path) {
		return fmt.Errorf("%w: %s", kvspace.ErrDirMustEndWithSlash, path)
	}
	if err := b.ensureSingleDirCachedLocked(kvspace.PathSep, "", kvspace.PathSep, indexes); err != nil {
		return err
	}
	if path == kvspace.PathSep {
		return nil
	}

	parent := kvspace.PathSep
	current := kvspace.PathSep
	for _, name := range strings.Split(strings.Trim(path, kvspace.PathSep), kvspace.PathSep) {
		current = kvspace.JoinPath(current, name) + kvspace.DirIndexSuf
		if err := b.ensureSingleDirCachedLocked(parent, name, current, indexes); err != nil {
			return err
		}
		parent = current
	}
	return nil
}

func (b *backend) ensureSingleDirCachedLocked(parent, name, path string, indexes indexCache) error {
	if v, ok := b.valueLocked(path); ok {
		if v.Kind() != kvspace.KindIndex && v.Kind() != kvspace.KindExtIndex {
			return fmt.Errorf("%w: %s", kvspace.ErrNotDir, path)
		}
		if name != "" {
			b.addChildCachedLocked(parent, indexChild(path, name), indexes)
		}
		return nil
	}
	if name != "" {
		if err := b.checkExtTargetMutationLocked(path, indexes, nil, false); err != nil {
			return err
		}
		if err := b.checkExtMutationLocked(path, kvspace.ErrExtWrite, indexes); err != nil {
			return err
		}
	}
	b.putValueLocked(path, kvspace.NewIndex(nil))
	if name != "" {
		b.addChildCachedLocked(parent, indexChild(path, name), indexes)
	}
	return nil
}

func (b *backend) valueLocked(key string) (kvspace.XValue, bool) {
	data, ok := b.store.tree.get([]byte(key))
	if !ok {
		return nil, false
	}
	return kvspace.DecodeXValueHead(data).Decode(), true
}

func (b *backend) linkTargetLocked(key string) (string, bool) {
	data, ok := b.store.tree.get([]byte(key))
	if !ok {
		return "", false
	}
	kind, raw, ok := encodedValueParts(data)
	if !ok || !bytes.Equal(kind, []byte(kvspace.KindLinkIndex)) {
		return "", false
	}
	return string(raw), true
}

func (b *backend) valueHasKindLocked(key, want string) bool {
	data, ok := b.store.tree.get([]byte(key))
	if !ok {
		return false
	}
	kind, _, ok := encodedValueParts(data)
	return ok && bytes.Equal(kind, []byte(want))
}

func (b *backend) putValueLocked(key string, value kvspace.XValue) {
	b.store.tree.Insert([]byte(key), value.Encode())
}

func (b *backend) readDirIndexLocked(dir string) []string {
	v, ok := b.valueLocked(dir)
	if !ok {
		return nil
	}
	switch v.Kind() {
	case kvspace.KindIndex:
		return normalizeIndex(indexChildren(v))
	case kvspace.KindExtIndex:
		children, _ := extIndexParts(v)
		return cloneStrings(children)
	default:
		return nil
	}
}

func (b *backend) addChildLocked(parent, name string) {
	indexes := make(indexCache)
	b.addChildCachedLocked(parent, name, indexes)
	b.flushIndexesLocked(indexes)
}

type cachedIndex struct {
	kind     string
	children []string
	extPath  string
	seen     map[string]struct{}
	dirty    bool
}

type indexCache map[string]*cachedIndex

func (b *backend) replaceDirectoryCachedLocked(
	path string,
	children []string,
	indexes indexCache,
) error {
	cached, ok := b.indexCachedLocked(path, indexes)
	if !ok {
		cached = &cachedIndex{
			kind: kvspace.KindIndex,
			seen: make(map[string]struct{}),
		}
		indexes[path] = cached
	}
	if cached.kind != kvspace.KindIndex {
		return fmt.Errorf("%w: %s has kind=%s", kvspace.ErrInvalidDirValue, path, cached.kind)
	}
	cached.children = children
	cached.seen = make(map[string]struct{}, len(cached.children))
	for _, child := range cached.children {
		cached.seen[child] = struct{}{}
	}
	cached.dirty = true
	return nil
}

func (b *backend) indexCachedLocked(path string, indexes indexCache) (*cachedIndex, bool) {
	if cached, ok := indexes[path]; ok {
		return cached, true
	}
	v, ok := b.valueLocked(path)
	if !ok {
		return nil, false
	}
	cached := &cachedIndex{kind: v.Kind()}
	switch v.Kind() {
	case kvspace.KindIndex:
		cached.children = normalizeIndex(indexChildren(v))
	case kvspace.KindExtIndex:
		cached.children, cached.extPath = extIndexParts(v)
		cached.children = cloneStrings(cached.children)
	default:
		return nil, false
	}
	cached.seen = make(map[string]struct{}, len(cached.children))
	for _, child := range cached.children {
		cached.seen[child] = struct{}{}
	}
	indexes[path] = cached
	return cached, true
}

func (b *backend) addChildCachedLocked(parent, name string, indexes indexCache) {
	if name == "" {
		return
	}
	cached, ok := b.indexCachedLocked(parent, indexes)
	if !ok {
		cached = &cachedIndex{
			kind: kvspace.KindIndex,
			seen: make(map[string]struct{}),
		}
		indexes[parent] = cached
	}
	if cached.kind != kvspace.KindIndex && cached.kind != kvspace.KindExtIndex {
		panic(fmt.Errorf("%w: %s", kvspace.ErrNotDir, parent))
	}
	if _, exists := cached.seen[name]; exists {
		return
	}
	cached.children = append(cached.children, name)
	cached.seen[name] = struct{}{}
	cached.dirty = true
}

func (b *backend) flushIndexesLocked(indexes indexCache) {
	for path, cached := range indexes {
		if !cached.dirty {
			continue
		}
		if cached.kind == kvspace.KindExtIndex {
			b.putValueLocked(path, kvspace.NewExtIndex(cached.children, cached.extPath))
		} else {
			b.putValueLocked(path, kvspace.NewIndex(cached.children))
		}
	}
}

func (b *backend) removeChildLocked(parent, name string) {
	if name == "" {
		return
	}
	v, ok := b.valueLocked(parent)
	if !ok {
		return
	}
	switch v.Kind() {
	case kvspace.KindIndex:
		b.putValueLocked(parent, kvspace.NewIndex(without(normalizeIndex(indexChildren(v)), name)))
	case kvspace.KindExtIndex:
		children, target := extIndexParts(v)
		b.putValueLocked(parent, kvspace.NewExtIndex(without(children, name), target))
	}
}

func (b *backend) checkExtMutationLocked(path string, sentinel error, indexes indexCache) error {
	if _, ok := b.valueLocked(path); ok {
		return nil
	}
	target, ok := b.extFallbackPathLocked(path)
	if !ok {
		return nil
	}
	if _, exists := b.valueLocked(target); exists {
		return fmt.Errorf("%w: %s", sentinel, path)
	}
	parent, name := parentName(target)
	name = indexChild(target, name)
	if indexes != nil {
		if targetIndex, ok := b.indexCachedLocked(parent, indexes); ok {
			if _, exists := targetIndex.seen[name]; exists {
				return fmt.Errorf("%w: %s", sentinel, path)
			}
			return nil
		}
	}
	for _, child := range b.readDirIndexLocked(parent) {
		if child == name {
			return fmt.Errorf("%w: %s", sentinel, path)
		}
	}
	return nil
}

type extBinding struct {
	localDir  string
	targetDir string
}

func (b *backend) extBindingsLocked() ([]extBinding, error) {
	bindings := make([]extBinding, 0, len(b.store.exts))
	for localDir, extPath := range b.store.exts {
		targetDir, err := b.resolvePathErrLocked(extPath)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, extBinding{localDir: localDir, targetDir: targetDir})
	}
	return bindings, nil
}

func (b *backend) validateExtTargetsLocked() error {
	bindings, err := b.extBindingsLocked()
	if err != nil {
		return err
	}
	for i, binding := range bindings {
		if strings.HasPrefix(binding.localDir, binding.targetDir) {
			return fmt.Errorf("%w: %s", kvspace.ErrExtCascade, binding.localDir)
		}
		if b.pathUnderExtIndexLocked(binding.targetDir) {
			return fmt.Errorf("%w: %s", kvspace.ErrExtCascade, binding.targetDir)
		}
		if !b.valueHasKindLocked(binding.targetDir, kvspace.KindIndex) {
			return fmt.Errorf("%w: %s", kvspace.ErrExtTarget, binding.targetDir)
		}
		for j, other := range bindings {
			if i == j {
				continue
			}
			if strings.HasPrefix(other.localDir, binding.targetDir) {
				return fmt.Errorf("%w: %s", kvspace.ErrExtCascade, other.localDir)
			}
		}
	}
	for _, binding := range bindings {
		if overlap := firstIndexCollision(
			b.readDirIndexLocked(binding.localDir),
			b.readDirIndexLocked(binding.targetDir),
		); overlap != "" {
			return fmt.Errorf("%w: %s", kvspace.ErrExtCollision, overlap)
		}
		if overlap := b.firstExactSubtreeCollisionLocked(binding.localDir, binding.targetDir); overlap != "" {
			return fmt.Errorf("%w: %s", kvspace.ErrExtCollision, overlap)
		}
	}
	return nil
}

func (b *backend) checkExtTargetMutationLocked(
	path string,
	indexes indexCache,
	replacement []string,
	replaceIndex bool,
) error {
	bindings, err := b.extBindingsLocked()
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if !strings.HasPrefix(path, binding.targetDir) {
			continue
		}
		relative := strings.TrimPrefix(path, binding.targetDir)
		if relative == "" {
			if replaceIndex {
				local, ok := b.indexCachedLocked(binding.localDir, indexes)
				if ok {
					for _, child := range replacement {
						if _, exists := local.seen[child]; exists {
							return fmt.Errorf("%w: %s", kvspace.ErrExtCollision, child)
						}
					}
				}
			}
			continue
		}
		name := strings.SplitN(relative, kvspace.PathSep, 2)[0]
		name = indexChild(relative, name)
		if indexes != nil {
			if local, ok := b.indexCachedLocked(binding.localDir, indexes); ok {
				if _, exists := local.seen[name]; exists {
					return fmt.Errorf("%w: %s", kvspace.ErrExtCollision, name)
				}
			}
		}
		local := kvspace.JoinPath(binding.localDir, relative)
		for local != binding.localDir {
			if _, exists := b.valueLocked(local); exists {
				return fmt.Errorf("%w: %s", kvspace.ErrExtCollision, name)
			}
			local = parentDir(local)
		}
	}
	return nil
}

func (b *backend) pathUnderExtIndexLocked(path string) bool {
	for dir := path; ; dir = parentDir(dir) {
		if b.valueHasKindLocked(dir, kvspace.KindExtIndex) {
			return true
		}
		if dir == kvspace.PathSep {
			return false
		}
	}
}

func (b *backend) firstExactSubtreeCollisionLocked(localDir, targetDir string) string {
	var collision string
	b.store.tree.RangePrefix([]byte(localDir), func(key, _ []byte) bool {
		relative := strings.TrimPrefix(string(key), localDir)
		if relative == "" {
			return true
		}
		if _, exists := b.store.tree.get([]byte(kvspace.JoinPath(targetDir, relative))); !exists {
			return true
		}
		collision = strings.SplitN(relative, kvspace.PathSep, 2)[0]
		return false
	})
	return collision
}

func firstIndexCollision(left, right []string) string {
	seen := make(map[string]struct{}, len(left))
	for _, child := range left {
		seen[child] = struct{}{}
	}
	for _, child := range right {
		if _, ok := seen[child]; ok {
			return child
		}
	}
	return ""
}

func (b *backend) collectTreeKeysLocked(prefix string) [][]byte {
	var doomed [][]byte
	addPrefix := func(candidate string) {
		b.store.tree.RangePrefix([]byte(candidate), func(key, _ []byte) bool {
			doomed = append(doomed, key)
			return true
		})
	}
	if isDir(prefix) {
		addPrefix(prefix)
		return doomed
	}
	if _, ok := b.store.tree.get([]byte(prefix)); ok {
		doomed = append(doomed, []byte(prefix))
	}
	addPrefix(prefix + kvspace.PathSep)
	return doomed
}

func (b *backend) extFallbackPathLocked(path string) (string, bool) {
	dir := path
	if !isDir(dir) {
		dir, _ = parentName(path)
	}
	for {
		if b.valueHasKindLocked(dir, kvspace.KindExtIndex) {
			value, _ := b.valueLocked(dir)
			_, extPath := extIndexParts(value)
			if extPath == "" {
				return "", false
			}
			target := b.resolvePathLocked(extPath)
			relative := strings.TrimPrefix(path, dir)
			if relative == "" {
				return target, true
			}
			return b.resolvePathLocked(kvspace.JoinPath(target, relative)), true
		}
		if dir == kvspace.PathSep {
			return "", false
		}
		dir = parentDir(dir)
	}
}

func (b *backend) entryExistsLocked(parent, name string) bool {
	if name == "" {
		return false
	}
	_, ok := b.store.tree.get([]byte(kvspace.JoinPath(parent, name)))
	return ok
}

func (b *backend) resolvePathLocked(path string) string {
	resolved, err := b.resolvePathErrLocked(path)
	if err != nil {
		panic(err)
	}
	return resolved
}

func (b *backend) resolvePathErrLocked(path string) (string, error) {
	seen := make(map[string]struct{}, 4)
	for hops := 0; ; {
		if _, exists := seen[path]; exists {
			return "", fmt.Errorf("%w: link cycle at %s", kvspace.ErrResolve, path)
		}
		seen[path] = struct{}{}
		resolved, changed := b.resolveOneLocked(path)
		if !changed {
			return resolved, nil
		}
		hops++
		if hops > maxLinkResolutions {
			return "", fmt.Errorf("%w: more than %d link resolutions", kvspace.ErrResolve, maxLinkResolutions)
		}
		path = resolved
	}
}

func (b *backend) resolveParentLocked(path string) string {
	dirSuffix := isDir(path) && path != kvspace.PathSep
	clean := path
	if dirSuffix {
		clean = strings.TrimSuffix(path, kvspace.DirIndexSuf)
	}
	parent, last := kvspace.SepPath(clean)
	if parent == clean {
		return path
	}
	parentPath := parent
	if parentPath != kvspace.PathSep {
		parentPath += kvspace.DirIndexSuf
	}
	resolved := b.resolvePathLocked(parentPath)
	result := kvspace.JoinPath(resolved, last)
	if dirSuffix {
		result += kvspace.DirIndexSuf
	}
	return result
}

func (b *backend) resolveOneLocked(path string) (string, bool) {
	if path == kvspace.PathSep {
		return path, false
	}
	parts := strings.Split(strings.Trim(path, kvspace.PathSep), kvspace.PathSep)
	current := kvspace.PathSep
	for i, part := range parts {
		current = kvspace.JoinPath(current, part)
		linkKey := current
		if i+1 < len(parts) || isDir(path) {
			linkKey += kvspace.DirIndexSuf
		}
		target, ok := b.linkTargetLocked(linkKey)
		if !ok {
			continue
		}
		if i+1 < len(parts) {
			resolved := kvspace.JoinPath(target, strings.Join(parts[i+1:], kvspace.PathSep))
			if isDir(path) && !isDir(resolved) {
				resolved += kvspace.DirIndexSuf
			}
			return resolved, true
		}
		return target, true
	}
	return path, false
}

func (b *backend) acquireQueue(key string) *notifyQueue {
	b.store.queuesMu.Lock()
	queue := b.store.queues[key]
	if queue == nil {
		queue = newNotifyQueue()
		b.store.queues[key] = queue
	}
	queue.acquire()
	b.store.queuesMu.Unlock()
	return queue
}

func (b *backend) resolvedQueue(key string) (string, *notifyQueue) {
	b.store.mu.RLock()
	defer b.store.mu.RUnlock()
	resolved := b.resolvePathLocked(key)
	return resolved, b.acquireQueue(resolved)
}

func (b *backend) releaseQueue(key string, queue *notifyQueue) {
	b.store.queuesMu.Lock()
	queue.release()
	if queue.idle() && b.store.queues[key] == queue {
		delete(b.store.queues, key)
	}
	b.store.queuesMu.Unlock()
}

type notifyQueue struct {
	mu     sync.Mutex
	values [][]byte
	wake   chan struct{}
	users  int
}

func newNotifyQueue() *notifyQueue {
	return &notifyQueue{wake: make(chan struct{})}
}

func (q *notifyQueue) acquire() {
	q.mu.Lock()
	q.users++
	q.mu.Unlock()
}

func (q *notifyQueue) release() {
	q.mu.Lock()
	q.users--
	q.mu.Unlock()
}

func (q *notifyQueue) idle() bool {
	q.mu.Lock()
	idle := q.users == 0 && len(q.values) == 0
	q.mu.Unlock()
	return idle
}

func (q *notifyQueue) push(value []byte) {
	q.mu.Lock()
	q.values = append(q.values, cloneBytes(value))
	close(q.wake)
	q.wake = make(chan struct{})
	q.mu.Unlock()
}

func (q *notifyQueue) pop(timeout time.Duration, cancel <-chan struct{}) ([]byte, bool) {
	var timer *time.Timer
	var timeoutC <-chan time.Time
	if timeout > 0 {
		timer = time.NewTimer(timeout)
		timeoutC = timer.C
		defer timer.Stop()
	}

	for {
		q.mu.Lock()
		if n := len(q.values); n > 0 {
			value := q.values[n-1]
			q.values[n-1] = nil
			q.values = q.values[:n-1]
			q.mu.Unlock()
			return cloneBytes(value), true
		}
		wake := q.wake
		q.mu.Unlock()

		select {
		case <-wake:
		case <-timeoutC:
			return nil, false
		case <-cancel:
			return nil, false
		}
	}
}

func (q *notifyQueue) clear() {
	q.mu.Lock()
	for i := range q.values {
		q.values[i] = nil
	}
	q.values = nil
	close(q.wake)
	q.wake = make(chan struct{})
	q.mu.Unlock()
}

func isDir(path string) bool {
	return strings.HasSuffix(path, kvspace.DirIndexSuf)
}

func (b *backend) begin() error {
	b.lifeMu.RLock()
	select {
	case <-b.done:
		b.lifeMu.RUnlock()
		return kvspace.ErrDisconnected
	default:
	}
	return nil
}

func (b *backend) mustBegin() {
	if err := b.begin(); err != nil {
		panic(err)
	}
}

func (b *backend) end() {
	b.lifeMu.RUnlock()
}

func validateAbsolutePath(path string) error {
	if path == "" || path[0] != kvspace.PathSep[0] || strings.Contains(path, "//") {
		return fmt.Errorf("%w: %q", kvspace.ErrInvalidPath, path)
	}
	for i := 0; i < len(path); i++ {
		if path[i] < 0x20 || path[i] == 0x7f {
			return fmt.Errorf("%w: %q", kvspace.ErrInvalidPath, path)
		}
	}
	if path == kvspace.PathSep {
		return nil
	}
	for _, part := range strings.Split(strings.Trim(path, kvspace.PathSep), kvspace.PathSep) {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%w: %q", kvspace.ErrInvalidPath, path)
		}
	}
	return nil
}

func validateDirPath(path string) error {
	if err := validateAbsolutePath(path); err != nil {
		return err
	}
	if !isDir(path) {
		return fmt.Errorf("%w: %s", kvspace.ErrDirMustEndWithSlash, path)
	}
	return nil
}

func validateChildKey(key string) error {
	if key == "" {
		return nil
	}
	if strings.HasPrefix(key, kvspace.PathSep) {
		return fmt.Errorf("%w: child %q", kvspace.ErrInvalidPath, key)
	}
	clean := strings.TrimSuffix(key, kvspace.DirIndexSuf)
	if err := validateIndexChild(clean); err != nil {
		return err
	}
	return nil
}

func validateIndexChild(child string) error {
	base := strings.TrimSuffix(child, kvspace.DirIndexSuf)
	if base == "" || strings.Contains(base, kvspace.PathSep) || base == "." || base == ".." {
		return fmt.Errorf("%w: child %q", kvspace.ErrInvalidPath, child)
	}
	for i := 0; i < len(base); i++ {
		if base[i] < 0x20 || base[i] == 0x7f {
			return fmt.Errorf("%w: child %q", kvspace.ErrInvalidPath, child)
		}
	}
	return nil
}

func encodeStoredValue(value kvspace.XValue) ([]byte, kvspace.XValue, error) {
	if kvspace.IsNone(value) {
		return []byte{}, kvspace.None{}, nil
	}
	encoded := value.Encode()
	decoded := kvspace.DecodeXValueHead(encoded).Decode()
	if decoded.Kind() != value.Kind() ||
		decoded.ArrayLen() != value.ArrayLen() ||
		!bytes.Equal(decoded.Encode(), encoded) {
		return nil, nil, kvspace.ErrInvalidValue
	}
	return encoded, decoded, nil
}

func indexChildren(value kvspace.XValue) []string {
	return value.(kvspace.Index).Children()
}

func extIndexParts(value kvspace.XValue) ([]string, string) {
	ext := value.(kvspace.ExtIndex)
	return ext.Children(), ext.ExtPath()
}

func mustValidPath(path string) {
	if err := validateAbsolutePath(path); err != nil {
		panic(err)
	}
}

func mustValidDirPath(path string) {
	if err := validateDirPath(path); err != nil {
		panic(err)
	}
}

func parentDir(path string) string {
	clean := strings.TrimSuffix(path, kvspace.DirIndexSuf)
	parent, _ := parentName(clean)
	return parent
}

func parentName(path string) (string, string) {
	if isDir(path) && path != kvspace.PathSep {
		path = strings.TrimSuffix(path, kvspace.DirIndexSuf)
	}
	parent, last := kvspace.SepPath(path)
	if parent != kvspace.PathSep {
		parent += kvspace.DirIndexSuf
	}
	return parent, last
}

func indexChild(path, name string) string {
	if name != "" && isDir(path) {
		return name + kvspace.DirIndexSuf
	}
	return name
}

func encodedValueParts(data []byte) (kind, raw []byte, ok bool) {
	if len(data) == 0 {
		return nil, nil, false
	}
	kindLen := int(data[0])
	headerLen := 1 + kindLen + 8
	if kindLen == 0 || len(data) < headerLen {
		return nil, nil, false
	}
	rawLen := int(binary.LittleEndian.Uint32(data[1+kindLen+4 : headerLen]))
	if rawLen < 0 || len(data) < headerLen+rawLen {
		return nil, nil, false
	}
	return data[1 : 1+kindLen], data[headerLen : headerLen+rawLen], true
}

func normalizeIndex(values []string) []string {
	if len(values) == 0 || len(values) == 1 && values[0] == "" {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func without(values []string, target string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}
