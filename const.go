package kvspace

import "errors"

// ── 路径结构 ────────────────────────────────────────────────────────────────

const (
	PathSep          = "/"  // 路径分隔符
	DirIndexSuf      = "/"  // 目录索引键后缀（尾斜杠 = 目录，必须以 / 开头的 key 保证不冲突）
	RuntimeMemberSep = "‥"  // 运行时保留字段前缀（U+2025）——‥ 的唯一定义处，List 时隐藏
	IndexValueSep    = "\n" // index XValueHead 中的路径分隔符
	ExtIndexHead     = "…"  // extindex XValueHead bytes 首元素前缀，如 …/lib/init/
)

var (
	ErrDirMustEndWithSlash = errors.New("kvspace: index must end with /")
	ErrInvalidPath         = errors.New("kvspace: path must be absolute and canonical")
	ErrInvalidDirValue     = errors.New("kvspace: directory value must be kind=index")
	ErrInvalidValue        = errors.New("kvspace: value cannot be encoded and decoded losslessly")
	ErrDisconnected        = errors.New("kvspace: connection is disconnected")
	ErrGet                 = errors.New("kvspace: GET")
	ErrPipeExec            = errors.New("kvspace: pipeline exec")
	ErrResolve             = errors.New("kvspace: 路径解析 GET")
	ErrScan                = errors.New("kvspace: SCAN")
	ErrExtWrite            = errors.New("kvspace: 禁止对 extindex 只读路径执行写操作")
	ErrExtDel              = errors.New("kvspace: 禁止删除 extindex 只读路径")
	ErrNotDir              = errors.New("kvspace: 父路径不是目录")
	ErrParentNotFound      = errors.New("kvspace: 父目录不存在")
	ErrExtCascade          = errors.New("kvspace: ExtIndex 不容许级联")
	ErrExtTarget           = errors.New("kvspace: ExtIndex target must be an existing ordinary index")
	ErrExtCollision        = errors.New("kvspace: ExtIndex local and extension children overlap")
	ErrLinkTypeMismatch    = errors.New("kvspace: Link target 和 linkpath 类型不一致")
	ErrLinkPathExists      = errors.New("kvspace: Link path already contains a non-link value")
)

// ── XValueHead kind ──────────────────────────────────────────────────────────────

const (
	KindNone = "None"
	DictSep  = "."

	// kind 继承树：byte 是所有定长数值类型的祖先。
	// 字节宽度由 ElemSize(kind) 定义——任何 elemSize>0 的 kind 继承 byte。
	KindByte       = "byte"       // 基础字节，elemSize=1
	KindBool       = "bool"       // → byte, 1B
	KindInt8       = "int8"       // → byte, 1B
	KindUint8      = "uint8"      // → byte, 1B
	KindStringByte = "stringbyte" // → byte, 1B×N (UTF-8)
	KindInt16      = "int16"      // → byte, 2B
	KindUint16     = "uint16"     // → byte, 2B
	KindInt32      = "int32"      // → byte, 4B
	KindUint32     = "uint32"     // → byte, 4B
	KindFloat32    = "float32"    // → byte, 4B
	KindInt64      = "int64"      // → byte, 8B
	KindUint64     = "uint64"     // → byte, 8B
	KindFloat64    = "float64"    // → byte, 8B

	KindDict     = "dict"
	KindIndex    = "index"
	KindExtIndex = "extindex" // 扩展索引，写留在上层

	KindRwir   = "rwir"   // 原子读写指令槽
	KindRwfunc = "rwfunc" // 函数定义（复合 rwir）
	KindScope  = "scope"  // 函数内 {} 作用域（lower 产物）
)
