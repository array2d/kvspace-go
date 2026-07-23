# Redis 操作统计

环境变量 `KVSPACE_REDIS_LOG=1` 开启命令日志，测量各 API 在不同路径深度下的 Redis 命令数。

路径深度: d1=`/a`, d2=`/a/b`, d4=`/a/b/c/d`, d8=`/a/b/c/d/e/f/g/h`

## Set

| 深度 | 顶层命令 | Pipeline 内容 | 总 ops |
|------|---------|--------------|--------|
| d1 | 1 SMEMBERS + 1 pipeline | sadd×2 + set×1 | 3 |
| d2 | 1 SMEMBERS + 1 pipeline | sadd×3 + set×1 | 4 |
| d4 | 1 SMEMBERS + 1 pipeline | sadd×5 + set×1 | 6 |
| d8 | 1 SMEMBERS + 1 pipeline | sadd×9 + set×1 | 10 |

公式: `1 + (depth + 2)`，其中 pipeline 含 depth+1 个 SADD（pipeIndex 逐层建索引）+ 1 个 SET。

## Get

| 深度 | 顶层命令 | Pipeline 内容 | 总 ops |
|------|---------|--------------|--------|
| 任意 | 2 SMEMBERS + 1 pipeline | get×1 | 3 |

**O(1)，与深度无关。**

Get(link) d8: 3 SMEMBERS + pipeline(1 get) = **4 ops**
Get(extindex) d8: 3 SMEMBERS + pipeline(1 get) = **4 ops**

## List

| 深度 | 顶层命令 | 总 ops |
|------|---------|--------|
| 任意 | 2 SMEMBERS | 2 |

**O(1)，与深度无关。**

## Del

| 深度 | 顶层命令 | 总 ops |
|------|---------|--------|
| d1 | 2 SMEMBERS + pipeline(2) + SCARD + SREM | 5 |
| d2 | + EXISTS + SCARD + SREM | 8 |
| d4 | + 3×(EXISTS + SCARD + SREM) | 14 |
| d8 | + 7×(EXISTS + SCARD + SREM) | 26 |

公式: `5 + 3×(depth−1)` = **O(depth)**

每级级联清理 (delIndex) 做 3 次调用: EXISTS(父节点) + SCARD(父/) + SREM(祖/ 移除父)。

## DelTree

| 深度 | 顶层命令 | 总 ops |
|------|---------|--------|
| d1 (含1子) | 3 SMEMBERS + 2 DEL + SCARD + SREM | 7 |
| d2 (含1子) | 3 SMEMBERS + 2 DEL + delIndex cascade | 10 |
| d4 (含1子) | 3 SMEMBERS + 2 DEL + delIndex cascade | 16 |
| d8 (含1子) | 3 SMEMBERS + 2 DEL + delIndex cascade | 28 |

公式: `7 + 3×(depth−1)` = **O(depth)**

3 SMEMBERS = 2(Get 定位) + 1(delRecursive 读子节点)。

## Link / ExtIndex

| 深度 | 顶层命令 | 总 ops |
|------|---------|--------|
| d1 | pipeline(2) + SADD | 2 |
| d2 | pipeline(2) + 2 SADD | 3 |
| d4 | pipeline(2) + 4 SADD | 5 |
| d8 | pipeline(2) + 8 SADD | 9 |

公式: `1 + (depth + 1)` = **O(depth)**

pipeline(2) = SET + SADD (.ext 条目)。额外 SADD 来自 addIndex，每个路径层级一次独立 SADD（未合并到 pipeline）。

## UnLink

| 深度 | 顶层命令 | 总 ops |
|------|---------|--------|
| d1 | pipeline(2) + SCARD + SREM | 3 |
| d2 | pipeline(2) + delIndex cascade | 6 |
| d4 | pipeline(2) + delIndex cascade | 12 |
| d8 | pipeline(2) + delIndex cascade | 24 |

公式: `3 + 3×(depth−1)` = **O(depth)**，同 Del 的级联清理。

## Set through Link (d8)

3 顶层命令: SMEMBERS + GET + pipeline(4)

pipeline(4) = SADD + SET + 2 SADD。其中有一个冗余 SADD（pipeIndex 和显式 SADD 重复添加同一 key）。

## 优化空间

### 1. Get/List/Del 的重复 SMEMBERS ⭐

`resolveChain` 和 `resolveSets` 各自独立调用 SMEMBERS 查同一 dirKey，导致 Get 浪费 1 次、List 浪费 1 次、Del 浪费 1 次 Redis 调用。

**修复**: 让 `resolveSets` 复用 `resolveChain` 内部的 SMEMBERS 结果，或改为先 GET XValue 判断 KindLink/KindExtIndex 再决定是否读 Set。

```go
// 当前: resolveChain → SMEMBERS; resolveSets → SMEMBERS (重复)
// 优化: 将 resolveChain 的结果集传入 resolveSets 复用
```

### 2. addIndex 的逐层独立 SADD ⭐

Link/ExtIndex/Set 调用 addIndex，对路径的每个 `/` 层级发一次独立 SADD。深度 d8 产生 8 次独立 SADD 调用。

**修复**: 将 addIndex 的 SADD 合并到 pipeline 中，或直接不单独维护——Set 的 pipeIndex 已经在 pipeline 里做了逐层 SADD，addIndex 对 Link/ExtIndex 是额外开销。

```go
// 当前: addIndex 逐层 r.rdb.SAdd() — N 次独立调用
// 优化: 合并到 Link/ExtIndex 自身的 pipeline 中
```

### 3. delIndex 级联的 EXISTS + SCARD 可合并 ⭐

每层级联清理做 EXISTS + SCARD 两次调用。但 SCARD=0 且 parent 无值时，无需 EXISTS。

**修复**: 先 SCARD（成本同 EXISTS），若 SCARD=0 则直接继续级联，省掉 EXISTS。

```go
// 当前: EXISTS(parent) → if true return; SCARD(parent/) → if >0 return
// 优化: 先 SCARD(parent/); 若=0 直接 SREM + 继续，省 EXISTS
```

### 4. Set through Link 的冗余 SADD

pipeIndex + 显式 SADD 都对 writeIdx 添加 last，产生一次重复 SADD（幂等，无功能影响但浪费）。

**修复**: 去掉显式 SADD，pipeIndex 已覆盖。

```go
// 当前:
pipe.SAdd(bg, writeIdx, last)       // 显式
pipeIndex(pipe, writePath)           // pipeIndex 内部也会 SADD writeIdx last
// 优化: 只保留 pipeIndex
```

### 5. 简单路径的 Get/List 可跳过 SMEMBERS

对无 extindex 的普通路径，`resolveChain` 的 SMEMBERS 必然返回无 `.ext`。此时可以先 GET XValue 判断 kind——非 KindLink/KindExtIndex 则跳过 SMEMBERS。GET 比 SMEMBERS 便宜（O(1) vs 可能 O(n)）。

**适用**: 大多数路径是普通目录，Link/ExtIndex 是少数。

### 优化优先级

| 优化项 | 节省 | 改动量 | 建议 |
|--------|------|--------|------|
| #1 重复 SMEMBERS | Get -33%, List -50% | 小 | 立即做 |
| #2 addIndex 合并 | Link/ExtIndex -40% | 小 | 立即做 |
| #3 delIndex 省 EXISTS | Del/DelTree/UnLink -17% | 小 | 可做 |
| #4 SetLink 冗余 SADD | SetLink -25% | 极小 | 可做 |
| #5 简单路径优化 | Get/List 高频场景 | 中 | 后续 |
