package kvspace
import "errors"
// ── 路径结构 ────────────────────────────────────────────────────────────────

const (
	PathSep        = "/"    // 路径分隔符
	DirIndexSuf    = "/"    // 目录索引键后缀（尾斜杠 = 目录，必须以 / 开头的 key 保证不冲突）
	ReservedPrefix = "."    // 引擎保留字段前缀，List 时隐藏
	IndexValueSep= "\n"    // index XValue 中的路径分隔符
	ExtIndexHead = "=" // extindex XValue bytes 首元素前缀，如 =/lib/init/
)

var (
	ErrDirMustEndWithSlash = errors.New("kvspace: index must end with /")
	ErrGet                 = errors.New("kvspace: GET")
	ErrPipeExec            = errors.New("kvspace: pipeline exec")
	ErrResolve             = errors.New("kvspace: 路径解析 GET")
	ErrScan                = errors.New("kvspace: SCAN")
	ErrExtWrite            = errors.New("kvspace: 禁止对 extindex 只读路径执行写操作")
	ErrExtDel              = errors.New("kvspace: 禁止删除 extindex 只读路径")
	ErrNotDir              = errors.New("kvspace: 父路径不是目录")
	ErrParentNotFound      = errors.New("kvspace: 父目录不存在")
	ErrExtCascade          = errors.New("kvspace: ExtIndex 不容许级联")
	ErrLinkTypeMismatch    = errors.New("kvspace: Link target 和 linkpath 类型不一致")
)
// ── XValue kind ──────────────────────────────────────────────────────────────

const (
	KindNull      = "null"
	KindBool      = "bool"
	KindInt8      = "int8"
	KindInt16     = "int16"
	KindInt32     = "int32"
	KindInt64     = "int64"
	KindUint8     = "uint8"
	KindUint16    = "uint16"
	KindUint32    = "uint32"
	KindUint64    = "uint64"
	KindFloat32   = "float32"
	KindFloat64   = "float64"
	KindString    = "string"
	KindBytes     = "bytes"
	KindArray1d   = "array1d"
	KindDict      = "dict"
	KindIndex     = "index"
	KindLinkIndex = "linkindex" // 纯链接，写穿透到目标
	KindExtIndex  = "extindex"  // 扩展索引，写留在上层
)
