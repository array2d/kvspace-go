package art

import "bytes"

type nodeHeader struct {
	prefix   []byte
	value    []byte
	hasValue bool
	count    int
}

type node interface {
	header() *nodeHeader
	child(byte) node
	eachChild(func(byte, node) bool)
	onlyChild() (byte, node)
}

type node4 struct {
	h        nodeHeader
	keys     [4]byte
	children [4]node
}

func (n *node4) header() *nodeHeader { return &n.h }
func (n *node4) child(key byte) node {
	for i := 0; i < n.h.count; i++ {
		if n.keys[i] == key {
			return n.children[i]
		}
	}
	return nil
}
func (n *node4) eachChild(fn func(byte, node) bool) {
	for i := 0; i < n.h.count; i++ {
		if !fn(n.keys[i], n.children[i]) {
			return
		}
	}
}
func (n *node4) onlyChild() (byte, node) {
	if n.h.count != 1 {
		return 0, nil
	}
	return n.keys[0], n.children[0]
}

type node16 struct {
	h        nodeHeader
	keys     [16]byte
	children [16]node
}

func (n *node16) header() *nodeHeader { return &n.h }
func (n *node16) child(key byte) node {
	for i := 0; i < n.h.count; i++ {
		if n.keys[i] == key {
			return n.children[i]
		}
	}
	return nil
}
func (n *node16) eachChild(fn func(byte, node) bool) {
	for i := 0; i < n.h.count; i++ {
		if !fn(n.keys[i], n.children[i]) {
			return
		}
	}
}
func (n *node16) onlyChild() (byte, node) {
	if n.h.count != 1 {
		return 0, nil
	}
	return n.keys[0], n.children[0]
}

type node48 struct {
	h        nodeHeader
	index    [256]uint8
	children [48]node
}

func (n *node48) header() *nodeHeader { return &n.h }
func (n *node48) child(key byte) node {
	slot := n.index[key]
	if slot == 0 {
		return nil
	}
	return n.children[int(slot)-1]
}
func (n *node48) eachChild(fn func(byte, node) bool) {
	for i, slot := range n.index {
		if slot != 0 && !fn(byte(i), n.children[int(slot)-1]) {
			return
		}
	}
}
func (n *node48) onlyChild() (byte, node) {
	if n.h.count != 1 {
		return 0, nil
	}
	for i, slot := range n.index {
		if slot != 0 {
			return byte(i), n.children[int(slot)-1]
		}
	}
	return 0, nil
}

type node256 struct {
	h        nodeHeader
	children [256]node
}

func (n *node256) header() *nodeHeader { return &n.h }
func (n *node256) child(key byte) node { return n.children[key] }
func (n *node256) eachChild(fn func(byte, node) bool) {
	for i, child := range n.children {
		if child != nil && !fn(byte(i), child) {
			return
		}
	}
}
func (n *node256) onlyChild() (byte, node) {
	if n.h.count != 1 {
		return 0, nil
	}
	for i, child := range n.children {
		if child != nil {
			return byte(i), child
		}
	}
	return 0, nil
}

// radixTree 不负责并发控制。
type radixTree struct {
	root node
}

func (t *radixTree) Get(key []byte) ([]byte, bool) {
	value, ok := t.get(key)
	if !ok {
		return nil, false
	}
	return cloneBytes(value), true
}

// get 返回内部切片，调用方不得修改或长期持有。
func (t *radixTree) get(key []byte) ([]byte, bool) {
	n := t.root
	depth := 0
	for n != nil {
		h := n.header()
		if !hasPrefixAt(key, depth, h.prefix) {
			return nil, false
		}
		depth += len(h.prefix)
		if depth == len(key) {
			if !h.hasValue {
				return nil, false
			}
			return h.value, true
		}
		if depth > len(key) {
			return nil, false
		}
		n = n.child(key[depth])
		depth++
	}
	return nil, false
}

func (t *radixTree) Insert(key, value []byte) {
	t.root = insertNode(t.root, key, 0, value)
}

func insertNode(n node, key []byte, depth int, value []byte) node {
	if n == nil {
		return newLeaf(key[depth:], value)
	}

	h := n.header()
	common := commonPrefix(h.prefix, key[depth:])
	if common < len(h.prefix) {
		parent := &node4{}
		parent.h.prefix = cloneBytes(h.prefix[:common])

		oldEdge := h.prefix[common]
		h.prefix = cloneBytes(h.prefix[common+1:])
		var out node = parent
		out = addChild(out, oldEdge, n)

		depth += common
		if depth == len(key) {
			parent.h.hasValue = true
			parent.h.value = cloneBytes(value)
		} else {
			newEdge := key[depth]
			out = addChild(out, newEdge, newLeaf(key[depth+1:], value))
		}
		return out
	}

	depth += len(h.prefix)
	if depth == len(key) {
		h.hasValue = true
		h.value = cloneBytes(value)
		return n
	}

	edge := key[depth]
	child := n.child(edge)
	if child == nil {
		return addChild(n, edge, newLeaf(key[depth+1:], value))
	}

	child = insertNode(child, key, depth+1, value)
	return replaceChild(n, edge, child)
}

func (t *radixTree) Delete(key []byte) {
	t.root, _ = deleteNode(t.root, key, 0)
}

func deleteNode(n node, key []byte, depth int) (node, bool) {
	if n == nil {
		return nil, false
	}
	h := n.header()
	if !hasPrefixAt(key, depth, h.prefix) {
		return n, false
	}
	depth += len(h.prefix)

	if depth == len(key) {
		if !h.hasValue {
			return n, false
		}
		h.hasValue = false
		h.value = nil
		return compactNode(n), true
	}
	if depth > len(key) {
		return n, false
	}

	edge := key[depth]
	child := n.child(edge)
	if child == nil {
		return n, false
	}
	var deleted bool
	child, deleted = deleteNode(child, key, depth+1)
	if !deleted {
		return n, false
	}
	if child == nil {
		n = removeChild(n, edge)
	} else {
		n = replaceChild(n, edge, child)
	}
	return compactNode(n), true
}

func (t *radixTree) RangePrefix(prefix []byte, fn func(key, value []byte) bool) {
	rangePrefixNode(t.root, prefix, 0, nil, fn)
}

func rangePrefixNode(n node, prefix []byte, depth int, path []byte, fn func([]byte, []byte) bool) bool {
	if n == nil {
		return true
	}
	h := n.header()
	remaining := len(prefix) - depth
	if remaining < 0 {
		remaining = 0
	}
	compareLen := len(h.prefix)
	if remaining < compareLen {
		compareLen = remaining
	}
	if !bytes.Equal(h.prefix[:compareLen], prefix[depth:depth+compareLen]) {
		return true
	}

	path = appendCopy(path, h.prefix...)
	depth += len(h.prefix)
	if depth >= len(prefix) {
		return walkNode(n, path, fn)
	}

	edge := prefix[depth]
	child := n.child(edge)
	if child == nil {
		return true
	}
	return rangePrefixNode(child, prefix, depth+1, appendCopy(path, edge), fn)
}

func walkNode(n node, path []byte, fn func([]byte, []byte) bool) bool {
	h := n.header()
	if h.hasValue && !fn(cloneBytes(path), cloneBytes(h.value)) {
		return false
	}
	keepGoing := true
	n.eachChild(func(edge byte, child node) bool {
		childPath := appendCopy(path, edge)
		childPath = appendCopy(childPath, child.header().prefix...)
		keepGoing = walkNode(child, childPath, fn)
		return keepGoing
	})
	return keepGoing
}

func newLeaf(prefix, value []byte) node {
	return &node4{h: nodeHeader{
		prefix:   cloneBytes(prefix),
		value:    cloneBytes(value),
		hasValue: true,
	}}
}

func addChild(n node, key byte, child node) node {
	switch cur := n.(type) {
	case *node4:
		if cur.h.count == len(cur.children) {
			n = grow4(cur)
			return addChild(n, key, child)
		}
		i := cur.h.count
		for i > 0 && cur.keys[i-1] > key {
			cur.keys[i] = cur.keys[i-1]
			cur.children[i] = cur.children[i-1]
			i--
		}
		cur.keys[i], cur.children[i] = key, child
		cur.h.count++
		return cur
	case *node16:
		if cur.h.count == len(cur.children) {
			n = grow16(cur)
			return addChild(n, key, child)
		}
		i := cur.h.count
		for i > 0 && cur.keys[i-1] > key {
			cur.keys[i] = cur.keys[i-1]
			cur.children[i] = cur.children[i-1]
			i--
		}
		cur.keys[i], cur.children[i] = key, child
		cur.h.count++
		return cur
	case *node48:
		if cur.h.count == len(cur.children) {
			n = grow48(cur)
			return addChild(n, key, child)
		}
		slot := firstFree(cur.children[:])
		cur.children[slot] = child
		cur.index[key] = uint8(slot + 1)
		cur.h.count++
		return cur
	case *node256:
		if cur.children[key] == nil {
			cur.h.count++
		}
		cur.children[key] = child
		return cur
	default:
		panic("art: unknown node type")
	}
}

func replaceChild(n node, key byte, child node) node {
	switch cur := n.(type) {
	case *node4:
		for i := 0; i < cur.h.count; i++ {
			if cur.keys[i] == key {
				cur.children[i] = child
				return cur
			}
		}
	case *node16:
		for i := 0; i < cur.h.count; i++ {
			if cur.keys[i] == key {
				cur.children[i] = child
				return cur
			}
		}
	case *node48:
		if slot := cur.index[key]; slot != 0 {
			cur.children[int(slot)-1] = child
			return cur
		}
	case *node256:
		if cur.children[key] != nil {
			cur.children[key] = child
			return cur
		}
	}
	panic("art: replace missing child")
}

func removeChild(n node, key byte) node {
	switch cur := n.(type) {
	case *node4:
		for i := 0; i < cur.h.count; i++ {
			if cur.keys[i] != key {
				continue
			}
			copy(cur.keys[i:], cur.keys[i+1:cur.h.count])
			copy(cur.children[i:], cur.children[i+1:cur.h.count])
			cur.h.count--
			cur.keys[cur.h.count] = 0
			cur.children[cur.h.count] = nil
			return cur
		}
	case *node16:
		for i := 0; i < cur.h.count; i++ {
			if cur.keys[i] != key {
				continue
			}
			copy(cur.keys[i:], cur.keys[i+1:cur.h.count])
			copy(cur.children[i:], cur.children[i+1:cur.h.count])
			cur.h.count--
			cur.keys[cur.h.count] = 0
			cur.children[cur.h.count] = nil
			if cur.h.count <= 4 {
				return shrink16(cur)
			}
			return cur
		}
	case *node48:
		slot := cur.index[key]
		if slot == 0 {
			return cur
		}
		cur.index[key] = 0
		cur.children[int(slot)-1] = nil
		cur.h.count--
		if cur.h.count <= 16 {
			return shrink48(cur)
		}
		return cur
	case *node256:
		if cur.children[key] == nil {
			return cur
		}
		cur.children[key] = nil
		cur.h.count--
		if cur.h.count <= 48 {
			return shrink256(cur)
		}
		return cur
	}
	return n
}

func compactNode(n node) node {
	h := n.header()
	if h.hasValue {
		return n
	}
	if h.count == 0 {
		return nil
	}
	if h.count != 1 {
		return n
	}
	edge, child := n.onlyChild()
	combined := make([]byte, 0, len(h.prefix)+1+len(child.header().prefix))
	combined = append(combined, h.prefix...)
	combined = append(combined, edge)
	combined = append(combined, child.header().prefix...)
	child.header().prefix = combined
	return child
}

func grow4(old *node4) node {
	n := &node16{h: cloneHeader(old.h)}
	for i := 0; i < old.h.count; i++ {
		n.keys[i], n.children[i] = old.keys[i], old.children[i]
	}
	return n
}

func grow16(old *node16) node {
	n := &node48{h: cloneHeader(old.h)}
	for i := 0; i < old.h.count; i++ {
		n.children[i] = old.children[i]
		n.index[old.keys[i]] = uint8(i + 1)
	}
	return n
}

func grow48(old *node48) node {
	n := &node256{h: cloneHeader(old.h)}
	old.eachChild(func(key byte, child node) bool {
		n.children[key] = child
		return true
	})
	return n
}

func shrink16(old *node16) node {
	n := &node4{h: cloneHeader(old.h)}
	for i := 0; i < old.h.count; i++ {
		n.keys[i], n.children[i] = old.keys[i], old.children[i]
	}
	return n
}

func shrink48(old *node48) node {
	n := &node16{h: cloneHeader(old.h)}
	i := 0
	old.eachChild(func(key byte, child node) bool {
		n.keys[i], n.children[i] = key, child
		i++
		return true
	})
	return n
}

func shrink256(old *node256) node {
	n := &node48{h: cloneHeader(old.h)}
	i := 0
	old.eachChild(func(key byte, child node) bool {
		n.children[i] = child
		n.index[key] = uint8(i + 1)
		i++
		return true
	})
	return n
}

func cloneHeader(h nodeHeader) nodeHeader {
	return nodeHeader{
		prefix:   cloneBytes(h.prefix),
		value:    cloneBytes(h.value),
		hasValue: h.hasValue,
		count:    h.count,
	}
}

func commonPrefix(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

func hasPrefixAt(key []byte, depth int, prefix []byte) bool {
	if depth < 0 || depth+len(prefix) > len(key) {
		return false
	}
	return bytes.Equal(key[depth:depth+len(prefix)], prefix)
}

func firstFree(children []node) int {
	for i, child := range children {
		if child == nil {
			return i
		}
	}
	panic("art: node has no free child slot")
}

func appendCopy(base []byte, values ...byte) []byte {
	out := make([]byte, len(base), len(base)+len(values))
	copy(out, base)
	return append(out, values...)
}

func cloneBytes(src []byte) []byte {
	if src == nil {
		return nil
	}
	out := make([]byte, len(src))
	copy(out, src)
	return out
}
