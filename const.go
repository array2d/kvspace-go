package kvspace

// ── 路径结构 ────────────────────────────────────────────────────────────────

const (
	PathSep     = "/" // 路径分隔符
	DirIndexSuf = "/" // 目录索引键后缀（尾斜杠 = 目录，必须以 / 开头的 key 保证不冲突）
)

// ── 链接 ─────────────────────────────────────────────────────────────────────

const LinkSentinel = "->"

// ── XValue kind ──────────────────────────────────────────────────────────────

const (
	KindNull   = "null"
	KindBool   = "bool"
	KindInt8   = "int8"
	KindInt16  = "int16"
	KindInt32  = "int32"
	KindInt64  = "int64"
	KindUint8  = "uint8"
	KindUint16 = "uint16"
	KindUint32 = "uint32"
	KindUint64 = "uint64"
	KindFloat32 = "float32"
	KindFloat64 = "float64"
	KindInt    = "int"
	KindFloat  = "float"
	KindString = "string"
	KindBytes  = "bytes"
	KindArray1d  = "array1d"
	KindDict   = "dict"
)
