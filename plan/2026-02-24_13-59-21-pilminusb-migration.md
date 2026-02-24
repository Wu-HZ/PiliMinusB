---
mode: plan
cwd: D:\Programs\pilipiliworker\piliMinusB
task: PiliMinusB — Bilibili 用户行为数据自托管迁移计划（专业版）
complexity: complex
tool: manual-structured-analysis
total_thoughts: 10
created_at: 2026-02-24T13:59:21+08:00
---

# Plan: PiliMinusB — Bilibili 用户行为数据自托管迁移

## 🎯 任务概述

将 PiliPlusW（Flutter 客户端）中依赖 Bilibili 官方登录态的「存储类」用户行为数据迁移到自建 Go/Gin 服务器，使稍后再看、观看历史、收藏夹、关注/追番等数据脱离 B 站平台存储。自建服务器模拟官方 API 的请求路径与响应格式，最小化客户端改动。

**当前进度**：Phase 0–2 已完成并上线（17 个路由端点、4 个数据库模型）。Phase 3–4 待实施。

## 📋 项目现状快照

### 技术栈

| 层级 | 选型 | 版本 |
|------|------|------|
| 服务端框架 | Go + Gin | Go 1.24 / Gin 1.11 |
| ORM | GORM | 1.31 |
| 数据库 | MySQL | 通过 gorm.io/driver/mysql |
| 鉴权 | JWT (HS256, 30 天有效期) | golang-jwt/jwt/v5 |
| 客户端 | Flutter + Dart | GetX 状态管理 |
| 客户端请求 | SelfRequest (Dio + JWT Bearer) | 基址 127.0.0.1:8091 |

### 已实现的服务端模块

| 模块 | 文件 | 路由数 | 状态 |
|------|------|--------|------|
| 鉴权 (Phase 0) | handler/auth.go | 2 | ✅ 已上线 |
| 稍后再看 (Phase 1) | handler/watch_later.go | 5 | ✅ 已上线 |
| 观看历史 (Phase 2) | handler/history.go | 10 | ✅ 已上线 |
| 收藏夹 (Phase 3) | — | 0 | ⏳ 待实施 |
| 关注/追番 (Phase 4) | — | 0 | ⏳ 待实施 |

### 已实现的数据库模型

| 模型 | 文件 | 用途 |
|------|------|------|
| User | model/user.go | 用户注册/登录 |
| WatchLater | model/watch_later.go | 稍后再看列表 |
| WatchHistory | model/watch_history.go | 观看历史记录 |
| UserSettings | model/watch_history.go | 用户配置（如历史暂停状态） |

### 客户端 SelfRequest 迁移状态

| HTTP 模块 | 方法数 | 已迁移到 SelfRequest | 待迁移 |
|-----------|--------|---------------------|--------|
| user.dart | ~30 | 10 (历史+稍后再看) | — |
| video.dart | ~40 | 3 (heartBeat/historyReport/medialistHistory) | — |
| fav.dart | 34 | 0 | Phase 3 全部 |
| follow.dart | 1 | 0 | Phase 4 |
| member.dart | ~28 | 0 | Phase 4 部分 (分组管理) |
| dynamics.dart | ~24 | 0 | Phase 4 部分 (followUp) |

---

## 📋 执行计划

### Phase 3：收藏夹迁移

**目标**：迁移用户视频收藏体系（文件夹 CRUD + 内容管理），含与稍后再看的交叉操作。

#### 3.1 服务端 — 数据库模型

新增两个 GORM 模型，创建文件 `server/model/favorite.go`：

**FavFolder 模型**

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint (PK) | 自增主键 |
| UserID | uint | 用户 ID |
| MediaID | int64 | 收藏夹 ID（对外暴露，自增分配） |
| Title | string | 收藏夹名称 |
| Cover | string | 封面 URL |
| Intro | string | 简介 |
| MediaCount | int | 内容数量（由触发器或应用层维护） |
| Ctime | int64 | 创建时间戳 |
| Mtime | int64 | 修改时间戳 |
| SortOrder | int | 排序权重 |
| IsDefault | int | 0:普通 1:默认收藏夹 |
| 唯一索引 | (UserID, MediaID) | |

**FavResource 模型**

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint (PK) | 自增主键 |
| UserID | uint | 用户 ID |
| MediaID | int64 | 所属收藏夹 |
| ResourceID | int64 | 视频 aid |
| ResourceType | int | 默认 2 (视频) |
| Title | string | 视频标题 |
| Cover | string | 封面 |
| Intro | string | 简介 |
| Duration | int | 时长(秒) |
| UpperMid | int64 | UP 主 mid |
| UpperName | string | UP 主名称 |
| Bvid | string | 视频 bvid |
| Pubtime | int64 | 发布时间 |
| FavTime | int64 | 收藏时间 |
| 唯一索引 | (UserID, MediaID, ResourceID) | |
| 普通索引 | (UserID, MediaID, FavTime DESC) | |

#### 3.2 服务端 — 路由端点

新增文件 `server/handler/favorite.go`，实现以下 16 个端点：

##### 收藏夹管理 (7)

| # | 方法 | 路径 | 处理函数 | 说明 |
|---|------|------|----------|------|
| 1 | GET | `/x/v3/fav/folder/created/list-all` | AllFavFolders | 获取全部文件夹（支持 rid 参数查询视频所属夹） |
| 2 | GET | `/x/v3/fav/folder/created/list` | ListFavFolders | 分页获取文件夹列表 |
| 3 | GET | `/x/v3/fav/folder/info` | FavFolderInfo | 获取单个文件夹详情 |
| 4 | POST | `/x/v3/fav/folder/add` | AddFavFolder | 创建新文件夹 |
| 5 | POST | `/x/v3/fav/folder/edit` | EditFavFolder | 编辑文件夹名称/简介 |
| 6 | POST | `/x/v3/fav/folder/del` | DelFavFolder | 删除文件夹及其内容 |
| 7 | POST | `/x/v3/fav/folder/sort` | SortFavFolder | 文件夹排序 |

##### 收藏内容管理 (7)

| # | 方法 | 路径 | 处理函数 | 说明 |
|---|------|------|----------|------|
| 8 | GET | `/x/v3/fav/resource/list` | ListFavResources | 分页获取文件夹内容（支持 keyword 搜索、order 排序） |
| 9 | POST | `/x/v3/fav/resource/batch-deal` | BatchDealFav | 批量收藏/取消（add_media_ids + del_media_ids） |
| 10 | POST | `/x/v3/fav/resource/unfav-all` | UnfavAll | 清空指定文件夹 |
| 11 | POST | `/x/v3/fav/resource/copy` | CopyFavResource | 复制收藏到另一文件夹 |
| 12 | POST | `/x/v3/fav/resource/move` | MoveFavResource | 移动收藏到另一文件夹 |
| 13 | POST | `/x/v3/fav/resource/clean` | CleanFavResource | 清理失效内容（自建服务器无失效概念，可空操作） |
| 14 | POST | `/x/v3/fav/resource/sort` | SortFavResource | 收藏内容排序 |

##### 与稍后再看交叉操作 (2)

| # | 方法 | 路径 | 处理函数 | 说明 |
|---|------|------|----------|------|
| 15 | POST | `/x/v2/history/toview/copy` | ToviewCopy | 稍后再看 → 复制到收藏夹 |
| 16 | POST | `/x/v2/history/toview/move` | ToviewMove | 稍后再看 → 移动到收藏夹（并从稍后再看删除） |

#### 3.3 服务端 — 核心逻辑要点

1. **MediaID 分配**：每个用户维护独立的自增 MediaID 序列。新建文件夹时取该用户当前最大 MediaID + 1。首个文件夹自动标记 is_default=1。
2. **batch-deal 原子性**：在同一事务中完成 add_media_ids（插入）和 del_media_ids（删除），失败时整体回滚。
3. **media_count 维护**：每次增删收藏内容后，通过 `UPDATE fav_folder SET media_count = (SELECT COUNT(*) FROM fav_resource WHERE ...)` 保持一致。
4. **元数据填充**：收藏时与 Phase 1/2 共用 `bilibili.FetchVideoInfo()` 获取视频元信息。
5. **排序实现**：sort 接口接收 `media_ids` 有序数组，按数组索引写入 sort_order。
6. **响应格式**：所有端点返回与 B 站官方一致的 JSON 结构，确保客户端 Model.fromJson() 无需修改。

#### 3.4 客户端改动

| 文件 | 改动 | 方法数 |
|------|------|--------|
| lib/http/api.dart | 无需修改（路径与官方一致，SelfRequest 使用相对路径） | 0 |
| lib/http/fav.dart | 以下方法从 Request() → SelfRequest()，移除 csrf/AppSign | ~15 |

**需迁移的 fav.dart 方法清单**（仅视频收藏相关）：

| 方法 | 说明 |
|------|------|
| `allFavFolders()` | 获取全部文件夹 |
| `userfavFolder()` | 分页获取文件夹 |
| `favFolderInfo()` | 获取文件夹详情 |
| `addOrEditFolder()` | 创建/编辑文件夹 |
| `deleteFolder()` | 删除文件夹 |
| `sortFavFolder()` | 排序文件夹 |
| `userFavFolderDetail()` | 获取文件夹内容 |
| `favVideo()` | 批量收藏/取消 |
| `unfavAll()` | 清空文件夹 |
| `copyOrMoveFav()` | 复制/移动收藏 + 稍后再看交叉 |
| `cleanFav()` | 清理失效 |
| `sortFav()` | 排序收藏内容 |
| `videoInFolder()` | 查询视频所属文件夹 |

**不迁移的方法**（与 B 站平台深度耦合）：

| 方法 | 原因 |
|------|------|
| favPugv/addFavPugv/delFavPugv | PUGV 付费课程 |
| favTopic/addFavTopic/delFavTopic/likeTopic | 话题系统 |
| favArticle/addFavArticle/delFavArticle | 专栏/动态 |
| userNoteList/noteList/delNote | 笔记系统 |
| favSeasonList/seasonFav/cancelSub | 合集/订阅（非视频收藏） |
| favFavFolder/unfavFavFolder | 收藏他人文件夹 |
| communityAction | 社区互动 |
| spaceFav | 空间收藏页 |

---

### Phase 4：关注 / 追番 / 订阅

**目标**：迁移用户的内容订阅关系，使关注/追番操作不再向 B 站发送请求。

#### 4.1 服务端 — 数据库模型

新增文件 `server/model/follow.go`，包含 4 个模型：

**Following 模型**

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint (PK) | 自增主键 |
| UserID | uint | 用户 ID |
| Mid | int64 | 被关注者 B 站 mid |
| Uname | string | 用户名 |
| Face | string | 头像 URL |
| Sign | string | 签名 |
| Special | int | 0:普通 1:特别关注 |
| Mtime | int64 | 关注时间 |
| 唯一索引 | (UserID, Mid) | |

**FollowTag 模型**

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint (PK) | 自增主键 |
| UserID | uint | 用户 ID |
| TagID | int64 | 分组 ID（自增分配） |
| Name | string | 分组名称 |
| 唯一索引 | (UserID, TagID) | |

**FollowTagMember 模型**

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint (PK) | 自增主键 |
| UserID | uint | 用户 ID |
| TagID | int64 | 分组 ID |
| Mid | int64 | 被关注者 mid |
| 唯一索引 | (UserID, TagID, Mid) | |

**BangumiFollow 模型**

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint (PK) | 自增主键 |
| UserID | uint | 用户 ID |
| SeasonID | int64 | 番剧 season_id |
| MediaID | int64 | 媒体 ID |
| Title | string | 标题 |
| Cover | string | 封面 |
| SeasonType | int | 1:番剧 2:电影 3:纪录片 |
| TotalCount | int | 总集数 |
| IsFinish | int | 0:连载 1:完结 |
| FollowStatus | int | 0:未标记 1:想看 2:在看 3:已看 |
| NewEpDesc | string | 最新集描述 |
| Progress | string | 进度描述 |
| Mtime | int64 | 时间戳 |
| 唯一索引 | (UserID, SeasonID) | |

#### 4.2 服务端 — 路由端点

新增文件 `server/handler/follow.go`，实现以下 14 个端点：

##### 关注管理 (10)

| # | 方法 | 路径 | 处理函数 |
|---|------|------|----------|
| 1 | GET | `/x/relation/followings` | Followings |
| 2 | GET | `/x/relation/followings/search` | SearchFollowings |
| 3 | GET | `/x/relation/tags` | FollowTags |
| 4 | GET | `/x/relation/tag` | FollowTagMembers |
| 5 | POST | `/x/relation/tag/create` | CreateFollowTag |
| 6 | POST | `/x/relation/tag/update` | UpdateFollowTag |
| 7 | POST | `/x/relation/tag/del` | DelFollowTag |
| 8 | POST | `/x/relation/tags/addUsers` | AddUsersToTag |
| 9 | POST | `/x/relation/tag/special/add` | SpecialAdd |
| 10 | POST | `/x/relation/tag/special/del` | SpecialDel |

##### 追番/追剧 (4)

| # | 方法 | 路径 | 处理函数 |
|---|------|------|----------|
| 11 | GET | `/x/space/bangumi/follow/list` | BangumiFollowList |
| 12 | POST | `/pgc/web/follow/add` | PgcFollowAdd |
| 13 | POST | `/pgc/web/follow/del` | PgcFollowDel |
| 14 | POST | `/pgc/web/follow/status/update` | PgcFollowStatusUpdate |

#### 4.3 服务端 — 核心逻辑要点

1. **关注操作完全脱离官方**：添加/移除关注不再向 B 站发送请求，纯本地数据操作。
2. **用户元信息获取**：添加关注时，可选调用 B 站公开 API `/x/web-interface/card?mid=xxx` 获取 uname/face/sign 并缓存。
3. **关注分组**：TagID 自增分配，分组删除时自动清理 FollowTagMember 关联。
4. **追番元数据**：添加追番时通过 B 站公开 pgc API 获取 title/cover/season_type 等信息。
5. **动态流兼容**：dynamics.dart 中的 `followUp()` 仍走 B 站 API（获取直播/在线状态），不迁移。仅 `followings()` 和分组管理迁移。

#### 4.4 客户端改动

| 文件 | 改动 |
|------|------|
| lib/http/follow.dart | `followings()` → SelfRequest |
| lib/http/member.dart | 分组相关 6 个方法 → SelfRequest：followUpTags, specialAction, addUsers, followUpGroup, createFollowTag, updateFollowTag, delFollowTag, getfollowSearch |
| lib/http/video.dart | `relationMod()` → SelfRequest |
| lib/http/video.dart | `pgcAdd()`, `pgcDel()`, `pgcUpdate()` → SelfRequest |
| lib/http/fav.dart | `favPgc()` → SelfRequest（追番列表查询） |
| lib/http/dynamics.dart | `followUp()` → 保持 Request()，不迁移 |
| lib/utils/request_utils.dart | `actionRelationMod()` → SelfRequest |

---

## ⚠️ 风险与注意事项

### 技术风险

| 风险 | 影响 Phase | 应对 |
|------|-----------|------|
| **batch-deal 部分失败** | 3 | 使用数据库事务保证原子性；返回实际成功/失败的 ID 列表 |
| **media_count 不一致** | 3 | 每次增删后重新 COUNT，不依赖增量更新 |
| **收藏夹 MediaID 冲突** | 3 | 在用户级别用 MAX(media_id)+1 分配，加数据库唯一约束兜底 |
| **关注添加缺少元信息** | 4 | 关注时异步查询 B 站用户信息接口并缓存，查询失败不阻塞关注操作 |
| **动态流与本地关注不同步** | 4 | 明确告知用户：动态流仍来自 B 站，本地关注列表仅用于 UI 展示和客户端内筛选 |

### 架构约束

1. **不可迁移功能**：点赞、投币、评论、弹幕、转发 — 这些需要在 B 站平台生效，不在迁移范围内。
2. **不迁移的收藏子类型**：PUGV 课程、话题、专栏文章、笔记 — 与 B 站内容系统深度耦合。
3. **核心原则**：自建服务器模拟官方 API 的请求路径与响应格式，最小化客户端改动。客户端仅将 `Request()` 替换为 `SelfRequest()` 并移除 csrf/AppSign，不改变方法签名和调用方式。

---

## 📋 实施路线图

```
Phase 0 ✅ ── Phase 1 ✅ ── Phase 2 ✅ ── Phase 3 ⏳ ── Phase 4 ⏳
 (鉴权)       (稍后再看)     (观看历史)     (收藏夹)      (关注/追番)
 2 路由        5 路由        10 路由       16 路由       14 路由
 1 模型        1 模型        2 模型        2 模型        4 模型
```

**Phase 3 建议子步骤**：
1. 创建 model/favorite.go + AutoMigrate
2. 实现收藏夹管理 7 个端点
3. 实现收藏内容管理 7 个端点
4. 实现稍后再看交叉操作 2 个端点
5. 客户端 fav.dart 迁移（~13 个方法 → SelfRequest）
6. 端到端联调测试

**Phase 4 建议子步骤**：
1. 创建 model/follow.go + AutoMigrate
2. 实现关注管理 10 个端点
3. 实现追番/追剧 4 个端点
4. 客户端 follow.dart + member.dart + video.dart + request_utils.dart 迁移
5. 端到端联调测试

---

## 📎 参考

- `PLAN.md:1-663` — 原始计划文档
- `server/main.go:1` — 路由注册与模型迁移
- `server/handler/auth.go:1` — Phase 0 鉴权实现
- `server/handler/watch_later.go:1` — Phase 1 稍后再看实现
- `server/handler/history.go:1` — Phase 2 观看历史实现
- `server/model/watch_later.go:1` — Phase 1 数据模型（含 ToBiliJSON 模式参考）
- `server/model/watch_history.go:1` — Phase 2 数据模型
- `server/bilibili/video.go:1` — B 站元数据代理查询（带内存缓存）
- `PiliMinusB/lib/http/self_request.dart:1` — SelfRequest 客户端实现
- `PiliMinusB/lib/http/fav.dart:1` — Phase 3 待迁移的 34 个方法
- `PiliMinusB/lib/http/follow.dart:1` — Phase 4 待迁移
- `PiliMinusB/lib/http/member.dart:1` — Phase 4 分组管理待迁移
- `PiliMinusB/lib/http/video.dart:1` — relationMod/pgc 相关待迁移
- `PiliMinusB/lib/utils/request_utils.dart:1` — actionRelationMod 待迁移
- `issues/2026-02-24_12-00-00-pilminusb-plan.csv` — Issues CSV 快照
