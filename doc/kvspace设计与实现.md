# KVSpace 设计与实现

## 第一部分：基础模型

### 1. 核心概念

KVSpace 是文件系统风格的 KV 存储抽象，Redis 实现。

路径 `/a/b` 的值存为 Redis key `/a/b`。

目录 `/a/` 存为 Redis Set，成员是 `/a` 下的直接子名。

`/` 是根目录，`/` 的 Set 存顶级条目名。

### 2. XValue 类型系统

XValue必须有kindl类型，包括kindnull类型。

XValue 是带 kind 标签的不可变值，TLV 编码存入 Redis。

### 3. Link / ExtIndex

Link(target, linkpath)
kind=KindLink,xvalue=存string即可"<targetpath>"
    如果是普通key，读写均跳转<targetpath>
    如果目录key，下级的读写访问，均跳转<targetpath>/

ExtIndex(path, extpath)
kind=KindExtIndex,其redis存字符串数组[".ext=<exttargetindex_path>"，"node1","node2","node3","node4",]，redis-impl直接用set
只容许有1个ext,数组实现下exttargetindex_path必须是首个。
extindex 不容许级联,exttargetindex必须是普通index

## 第二部分：操作与索引

### 4. 路径解析

`ResolveExt(dirKey, getSet)` 从 Set 中查 `.ext` 条目，返回 ext 目标路径。

内部 `resolveSets` 做一次 SMEMBERS，查到 ext 才读第二层 Set。

### 5. 核心操作


## 第三部分：辅助设施

### 7. Watch / Notify

redis-impl
Watch/Notify 用 Redis BLPOP/LPUSH 实现一次性通知，link 路径穿透解析。

### 8. 编码与工具函数

`StripDirSuf` 去目录索引尾 `/`，`JoinPath` 拼路径避免 `//`，`SepPath` 拆路径为前缀+末段。

### 9. Redis 日志

Redis 日志由 `KVSPACE_REDIS_LOG` 控制等级：1=命令名，2=完整参数+耗时。

go-redis Hook 在每条命令前后记录，pipeline 显示批次数和总耗时。

### 10. 测试与构建

tutorial/ 下的 .sh 脚本头部 `# expected:` 注释预期输出，test.py 自动执行并对比。

`make build` 编译 kvspace 到 `~/.local/bin/kvspace`。