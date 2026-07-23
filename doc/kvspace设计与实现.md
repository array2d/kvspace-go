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

普通xvalue结构定义简单，请直接看代码

index类型的kind，value=string数组,redis-impl都用set直接实现

KindIndex     = "index"，
KindLinkIndex = "linkindex" // 纯链接，写穿透到目标
KindExtIndex  = "extindex"  // 

### 3. Link / ExtIndex

Link(target, linkpath)，link可以理解为文件系统软链接
kind=KindLink,xvalue=存string即可"targetpath"，link来说，target和linkpath要么都是值key，要么都是目录key以/结尾。


ExtIndex(path, extpath)，ExtIndex可以理解为写时复制的叠加层
    只能是目录,如extindex="/a/a2/",exttargetindex="/lib/funca/"，下级key(如如 "/a/a2/be")的读写访问，均跳转/lib/funca/be
ExtIndex是index，exttargetindex也是index，key必须都以/结尾
key不容许在2者重复！（extIndex自身存在的key，不容许出现在exttargetindex中）所以读操作覆盖二者，下级成员的写/创建/删除操作都只更新extIndex自身
逻辑上，我们通常不对extIndex/下的只读路径执行任何写操作，一旦操作，需要直接panic，警告开发者，强制修改
kind=KindExtIndex,xvalue存字符串数组的bytes["-->exttargetindex_path"，"node1","node2","node3","node4",]，对redis-impl直接用set
只容许有1个exttargetindex,数组实现下exttargetindex_path必须是首个。
extindex 不容许级联,exttargetindex必须是普通index
-->分隔符定义在const文件！

## 第二部分：操作与索引

完成后补充

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

### 11.严禁hardcode
不容许grep到乱丢的字符串，必须集中在const.go