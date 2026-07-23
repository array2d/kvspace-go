# KVSpace 设计与实现

## 第一部分：基础模型

### 1. 核心概念


+ 值key：也叫文件，普通key
+ 索引key：也叫目录，index，包括extindex。其xvalue内存储着下级目录成员列表，redis用set实现，其它impl是数组实现。

KVSpace 是文件系统风格的 KV 存储抽象，Redis 实现。

路径 `/a/b` 的值存为 Redis key `/a/b`。

目录（index，包括extindex），必须以/结尾 `/a/` 存为 Redis Set，成员是 `/a` 下的直接子名。

link可以是目录和文件key

/a 是 string key，/a/ 是 Set，独立共存

`/` 是根目录，`/` 的 Set 存顶级条目名。

### 2. XValue 类型系统

XValue必须有kind类型，包括kindnull类型。

XValue 是带 kind 标签的不可变值，TLV 编码存入 Redis。

TLV 编码格式。实际是 [1B kind_len][N B kind_name][4B arraylength LE][4B raw_len LE][M B raw]

xvalue结构定义简单，请直接看代码


### 3. Link / ExtIndex

Link(target, linkpath)，link可以理解为软链接
kind=KindLink,xvalue=存string即可"targetpath"
    如果是普通key如target="/a/a1",linkpath="/b/b1"，读写均跳转"/b/b1"
    如果目录,如extindex="/a/a2/",exttargetindex="/lib/funca/"，下级key(如如 "/a/a2/be")的读写访问，均跳转/lib/funca/be

ExtIndex(path, extpath)，ExtIndex可以理解为写时复制的叠加层
key不容许在2者重复！（extIndex自身存在的key，不容许出现在exttargetindex中）所以读操作覆盖二者，下级成员的写/创建/删除操作都只更新extIndex自身
逻辑上，我们通常不对extIndex/下的只读路径执行任何写操作，一旦操作，需要直接报错
kind=KindExtIndex,xvalue存字符串数组的bytes["->exttargetindex_path"，"node1","node2","node3","node4",]，对redis-impl直接用set
只容许有1个ext,数组实现下exttargetindex_path必须是首个。
extindex 不容许级联,exttargetindex必须是普通index

## 第二部分：操作与索引

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