# ExtIndex 设计

## 1. 概念模型

每个路径 `p` 在 KV 空间中有三层身份，由 **XValue 的 Kind** 唯一判定：

| Kind | 含义 | `/` Set 内容 | 写目标 | 读顺序 |
|------|------|-------------|--------|--------|
| 无 Kind 或有值 | 普通键/目录 | `{子键...}` | 自身 | 自身 |
| **KindExtIndex** `raw=""` 或无 raw | **LinkIndex**: 纯链接 | 空（或无 Set） | 扩展路径 | 扩展路径 |
| **KindExtIndex** `raw="<extpath>"` 且有本地条目 | **ExtIndex**: 扩展索引 | `{本地键, .ext=<extpath>/}` | 自身 | 自身 → 扩展路径 |

核心思想：**目录索引 Set 内部存储扩展引用 `.ext=<path>/`，索引链式查找替代路径前缀重写。**

Link 和 Ext 统一为 KindExtIndex，区别仅在于本地 Set 是否为空：

- **LinkIndex**: Set 为空 → 所有读写穿透到 extpath
- **ExtIndex**: Set 非空 → 本地条目优先，不命中回落 extpath

## 2. XValue Kind

```go
const (
    KindExtIndex = "extindex"  // 路径持有 extindex 引用，raw = extpath
    KindIndex    = "index"     // 目录索引（概念性，对应 `/` Set 存在）
)
```

- `KindExtIndex` 存储在**路径本身的 key** 上（如 `/link`），raw 编码扩展目标路径（至少含 `/` 结尾的 dir 索引路径）
- `KindIndex` 是概念性标记——目录索引由 `/` Set 存在性隐式表达，无需单独存储 XValue
- `KindMount` / `KindOverlay` 移除

### NewExtIndexValue / DecodeExtIndex

```go
func NewExtIndexValue(extpath string) XValue {
    return Raw(KindExtIndex, []byte(extpath))
}

func DecodeExtIndex(v XValue) string {
    if v.Kind() != KindExtIndex { return "" }
    return string(v.RawBytes())
}
```

## 3. 编码：`.ext` 在目录 Set 中

目录索引 Set `path/` (= `path + DirIndexSuf`) 中，用保留前缀条目存储 extindex 引用：

```
.ext=<ext_index_path>/     # 如 .ext=/target/、.ext=/lower/
```

已有 `ReservedPrefix = "."` 机制，`List` 天然过滤。

### 示例

```
# LinkIndex: /link → /target
/link       => XValue{kind: extindex, raw: "/target/"}
/link/      => {}                                 # 空 Set

# ExtIndex: /app extends /base
/app        => XValue{kind: extindex, raw: "/base/"}
/app/       => {".ext=/base/", "override_key", "new_key"}
/base/      => {"base_key1", "base_key2"}

# 普通目录: /data
/data       => XValue{kind: dict} 或任何其他值
/data/      => {"child1", "child2"}
```

### 为什么 XValue + Set 都要存 extpath？

- **XValue (`/link` → KindExtIndex raw="/target/")**：O(1) 判断"这个路径是不是 extindex"，无需读 Set
- **Set 中 `.ext` 条目**：List/Get 时，合并命名空间时直接遍历 Set 成员即可，无需额外 GET XValue

两者不矛盾——Set 是操作层索引，XValue 是元数据层标记。

## 4. API

```go
type KVSpace interface {
    // ── 读写（不变）────────────────────────────────────────────────
    Get(prefix string, keys []string) []XValue
    Set(pairs []KVPair) error
    List(prefix string) []string
    Del(keys ...string) error
    DelTree(prefix string) error
    Watch(key string, timeout time.Duration) XValue
    Notify(key string, val XValue) error

    // ── ExtIndex 系统（替代 Mount/Overlay/UnMount）─────────────────
    Link(target, linkPath string) error     // linkPath → target，纯链接
    ExtIndex(path, extPath string) error    // path 扩展 extPath，path 为写层
    UnLink(path string) error              // 移除 extindex

    Clear() error
    DisConn() error
}
```

### Link(target, linkPath)

```
Link("/real", "/alias")

/alias  => XValue{kind: extindex, raw: "/real/"}
/alias/ => {}  # 空 Set，所有操作穿透到 /real
```

- 写 `/alias/x` → 写 `/real/x`，`SADD /real/ x`
- 读 `/alias/x` → `/alias/` 空，直接查 `/real/` → 命中 → GET `/real/x`
- List `/alias` → `/alias/` 空，直接返回 List(`/real`)

### ExtIndex(path, extPath)

```
ExtIndex("/app", "/base")

/app   => XValue{kind: extindex, raw: "/base/"}
/app/  => {".ext=/base/"}  # 初始无本地条目
```

- 写 `/app/x` → 写 `/app/x`，`SADD /app/ x`（本地优先）
- 读 `/app/x` → `/app/` 有 x（本地命中）→ GET `/app/x`
- 读 `/app/y` → `/app/` 无 y → 跟 ext → `/base/` 有 y → GET `/base/y`
- List `/app` → `/app/` ∪ `/base/`，去重，本地优先

### UnLink(path)

- 若 path 的 XValue 为 KindExtIndex：
  - LinkIndex: 直接删 `/path` + `/path/`
  - ExtIndex: 删 `/path` + `/path/`（只删上层，extpath 不受影响）
- 若 path 无 KindExtIndex → no-op 或报错

## 5. 核心操作：ResolveExt

extindex 不容许级联——ext 目标必须是普通 index，深度永远 ≤2。因此只需一步查找：

```go
// ResolveExt 从 dirKey 的 Set 中解析 extindex 引用。
// 返回 (ext_target, 是否有 ext)。ext_target 以 / 结尾。
func ResolveExt(dirKey string, getSet func(string) []string) (string, bool) {
    for _, m := range getSet(dirKey) {
        if e := DecodeExtEntry(m); e != "" {
            return e, true
        }
    }
    return "", false
}
```

内部用 `resolveChain` 包装为 `[self]` 或 `[self, ext]` 以便各操作统一访问。

### 各操作使用 resolveChain

#### Get(prefix, keys)

```
chain, sets = resolveSets(dirKey(prefix))   # chain: [self] 或 [self, ext]
for each key:
    for i, idx in chain:
        if key in sets[i]:
            return GET(JoinPath(StripDirSuf(idx), key))
    → null
```

#### Set(pairs)

```
for each pair:
    prefix, last = SepPath(key)
    chain = resolveChain(dirKey(prefix))
    
    # 判定写目标
    if isPureLink(chain[0]):      # 第一层 Set 为空（或只有 .ext）
        writeIdx = chain[len(chain)-1]   # 写到终端层
    else:
        writeIdx = chain[0]              # 写到当前层（upper）
    
    writePath = writeIdx[:len(writeIdx)-1] + "/" + last
    SADD writeIdx last
    SET writePath val
```

**纯链接判定**：`getSet(chain[0])` 中无非 `.ext` 条目 → 这是 LinkIndex。

#### List(prefix)

```
chain = resolveIndexChain(prefix+"/")
merged = {}
for idx in chain:
    for m in getSet(idx):
        if not HasPrefix(m, ".") and m not in merged:
            merged.add(m)
return merged
```

链式遍历，天然去重，上层优先。

#### Del(keys...)

POSIX rm 语义：末段不穿透 extindex。

```
for each key:
    prefix, last = SepPath(key)
    
    # 先判定 prefix 是不是 extindex
    xv := GET(prefix)
    if xv.Kind() == KindExtIndex:
        if isPureLink(prefix+"/"):
            # 链接：写操作穿透，删操作也穿透——末段在终端删
            chain = resolveIndexChain(prefix+"/")
            targetIdx = chain[len(chain)-1]
            targetKey = targetIdx[:len(targetIdx)-1] + "/" + last
            DEL targetKey
            SREM targetIdx last
        else:
            # ExtIndex：末段作用于上层
            DEL prefix+"/"+last
            SREM prefix+"/" last
    else:
        # 普通路径：正常删除
        DEL prefix+"/"+last
        SREM prefix+"/" last
```

实际上，Del 的"末段不穿透"指的是：当 key 是 `/alias/x` 且 `/alias` 是 link 时，要删 `/real/x`，而不是删 `/alias/x`（因为 `/alias/x` 不存在——它只是透明路径）。但如果 key 就是 `/alias`，则删链接本体。

这有两种理解，需要明确：
- **访问 `/alias/x`**：path 解析后是 `/real/x`，Del 应该作用于 `/real/x`。因为从用户视角，`/alias/x` 就是 `/real/x`。
- **访问 `/alias`**：path 就是链接本身，Del 删除链接。

当前的 `ResolveParent` 实现是：`Del("/alias/x")` 穿透祖先链接 `/alias`，末段 `/x` 保留在结果路径中 → 删 `/real/x`。`Del("/alias")` 末段就是 `/alias`，不穿透。

ExtIndex 模型下，这个语义更自然：
- `Del("/alias/x")` → prefix=`/alias` 是 ExtIndex → 沿链找到 `/x` 的归属层 → 在那层删除
- `Del("/alias")` → 整个 key 就是 `/alias` → 删 XValue + `/alias/` Set

#### DelTree(prefix)

```
xv := GET(prefix)
if xv.Kind() == KindExtIndex:
    # 删除 extindex 本身（不穿透）
    if isPureLink(prefix+"/"):
        DEL prefix, DEL prefix+"/"
    else:
        delRecursive(prefix)   # 删上层数据
        DEL prefix+"/"
else:
    delRecursive(prefix)
    DEL prefix+"/"
```

## 6. 不再需要的组件

| 移除 | 原因 |
|------|------|
| `linkEntry` struct | 信息全在 Set + XValue 中 |
| `r.links map` | 无进程缓存 |
| `r.linkMu` | 无并发 map 访问 |
| `checkLink()` | XValue Kind 判断替代 |
| `checkLinkEntry()` | 同上 |
| `resolveOL()` | ResolveExt 替代 |
| `ResolveCore()` | ResolveExt 替代 |
| `ResolveParent()` | Del 沿索引链查找归属层 |
| `KindMount` / `KindOverlay` | KindExtIndex 统一 |
| `NewMountValue` / `DecodeMount` | NewExtIndexValue / DecodeExtIndex |
| `NewOverlayValue` / `DecodeOverlay` | 同上 |
| `OverlaySep` | 不再需要 `:` 分隔符 |
| Overlay 三参数 API | ExtIndex 二参数 |
| `ExtMaxHops` / 环检测 | ext 不容许级联，深度 ≤2 |

## 7. 改动文件清单

比 ResolveCore 的逐边界检查更高效——只沿 extindex 链跳转，不扫描路径中每个 `/`。

## 8. 改动文件清单

```
kvspace-go/
├── const.go              # 修改: +KindExtIndex, +KindIndex, -KindMount, -KindOverlay, -OverlaySep
├── kvspace.go            # 修改: API -Overlay +ExtIndex, Mount→Link
├── xvalue_keytree.go     # 修改: NewExtIndexValue/DecodeExtIndex 替代 Mount/Overlay
├── resolve.go            # 修改: ResolveCore→resolveIndexChain, ResolveParent→简化
├── redis/
│   └── redis.go          # 重写: linkEntry/links/resolveOL 全移除, 所有操作走索引链
└── cmd/kvspace/main.go   # 修改: overlay 子命令 → extindex
```

## 9. 确认的设计选择

1. **API 命名**：`Link` + `ExtIndex` + `UnLink` ✓
2. **ExtIndex 写语义**：merge=write，ExtIndex(path, extPath) 二参数 ✓
3. **`.ext` 分隔符**：定义在 const.go 中 `ExtIndexSep`，方便后续修改。当前值 `=` ✓
4. **KindIndex 存储**：不显式存 XValue——`/` Set 存在即隐含 KindIndex ✓
