# PiliMinusB 开发计划

## 一、项目概述

**目标**：将 PiliPlusW 中依赖 Bilibili 官方登录态的"存储类"服务迁移到自建服务器，
使用户的个人数据（稍后再看、收藏、观看历史、关注、订阅等）脱离 Bilibili 平台存储，
从而降低隐私泄露和大数据画像的风险。

**核心原则**：
- 自建服务器模拟官方 API 的请求路径与响应格式，最小化客户端改动
- 视频内容本身仍从 Bilibili 获取，仅迁移"用户行为数据"
- 分阶段实施，每阶段独立可用

**不可迁移的功能**（效果需在 B 站平台生效）：
- 点赞（影响 UP 主数据和推荐权重）
- 投币（UP 主收入）
- 评论 / 弹幕（需被其他用户看到）
- 转发 / 分享（社交传播链）

---

## 二、整体架构

```
┌─────────────────────────────────────────────────────────┐
│  PiliPlusW 客户端 (Flutter)                              │
│                                                         │
│  lib/http/constants.dart  ← 新增 selfBaseUrl             │
│  lib/http/api.dart        ← 按功能分流: self / official   │
│  lib/http/init.dart       ← 新增 SelfRequest 实例        │
└────────────┬──────────────────────────┬─────────────────┘
             │ 存储类请求                │ 交互类请求
             ▼                          ▼
┌────────────────────────┐  ┌──────────────────────────┐
│  PiliMinusB 自建服务器   │  │  Bilibili 官方 API        │
│                        │  │  api.bilibili.com         │
│  - 稍后再看 CRUD        │  │  - 点赞/投币/评论          │
│  - 收藏夹 CRUD          │  │  - 视频流/播放地址         │
│  - 观看历史 + 进度       │  │  - 搜索/推荐               │
│  - 关注列表             │  │  - 登录/鉴权               │
│  - 追番/订阅            │  │  - ...                     │
│                        │  │                           │
│  自有鉴权 (JWT/Token)   │  │  Cookie + CSRF            │
│  数据库 (SQLite/PG)     │  │                           │
└────────────────────────┘  └──────────────────────────┘
```

---

## 三、自建服务器通用设计

### 3.1 技术选型（建议）

| 层级     | 选项                              | 说明                         |
| -------- | --------------------------------- | ---------------------------- |
| 语言框架 | Go (Gin) / Node (Express) / Rust  | 按个人偏好选择                |
| 数据库   | SQLite（单用户轻量）/ PostgreSQL   | 初期 SQLite 即可              |
| 鉴权     | JWT Bearer Token                  | 替代 B 站的 Cookie + CSRF     |
| 部署     | 本地 / VPS / Docker               | 按需选择                      |

### 3.2 通用响应格式

所有接口统一返回与 Bilibili 相同的 JSON 包装结构，客户端无需修改解析逻辑：

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

错误时 `code != 0`，`message` 携带错误描述。

### 3.3 鉴权机制

客户端改造：
- 新建 `SelfRequest` 类（或在 `Request` 中添加拦截器），在请求头中附加：
  ```
  Authorization: Bearer <jwt_token>
  ```
- 所有发往自建服务器的 POST 请求不再携带 `csrf` 字段（服务端忽略即可）
- 所有发往自建服务器的 GET 请求不再需要 WbiSign 签名（服务端忽略即可）

服务端：
- 提供 `/auth/register` 和 `/auth/login` 接口
- JWT 中携带 `user_id`，用于关联用户数据

### 3.4 客户端公共改动

**文件：`lib/http/constants.dart`**
```dart
// 新增
static const String selfBaseUrl = 'https://your-server.com'; // 或本地地址
```

**文件：`lib/http/init.dart`**
- 新建一个针对自建服务器的 `Request` 实例或 Dio 拦截器
- 自动附加 JWT 鉴权头
- 不走 WbiSign

---

## 四、分阶段实施计划

---

### Phase 0：基础设施搭建

**目标**：建立自建服务器骨架和客户端请求通道。

#### 服务端任务

| # | 任务                          | 说明                                    |
|---|-------------------------------|-----------------------------------------|
| 1 | 项目初始化                     | 选定框架，搭建项目骨架                    |
| 2 | 数据库连接与 ORM 配置           | 配置 SQLite/PG 连接                      |
| 3 | 用户注册/登录接口               | POST `/auth/register`, `/auth/login`    |
| 4 | JWT 中间件                    | 验证 Authorization header                |
| 5 | 通用错误处理                   | 统一 `{"code": x, "message": "..."}` 格式 |

#### 客户端任务

| # | 任务                              | 涉及文件                         |
|---|-----------------------------------|----------------------------------|
| 1 | `constants.dart` 新增 selfBaseUrl  | `lib/http/constants.dart`        |
| 2 | 新增 SelfRequest 或 Dio 拦截器     | `lib/http/init.dart`（新增）      |
| 3 | 新增自建服务器登录/注册 UI          | 新页面                            |
| 4 | 本地存储 JWT Token                 | `lib/utils/storage.dart`          |

---

### Phase 1：稍后再看（Watch Later）

**目标**：最简单的迁移对象，作为验证整个方案可行性的试点。

#### 1.1 需要复制的官方 API

| 操作       | 方法 | 官方路径                           | 请求参数                                     |
|-----------|------|-----------------------------------|---------------------------------------------|
| 获取列表   | GET  | `/x/v2/history/toview/web`        | `pn, ps(20), viewed, key, asc, need_split`  |
| 添加视频   | POST | `/x/v2/history/toview/add`        | `aid, bvid`                                 |
| 删除视频   | POST | `/x/v2/history/toview/v2/dels`    | `resources`（逗号分隔的 aid）                  |
| 清空列表   | POST | `/x/v2/history/toview/clear`      | `clean_type`（null/1/2）                     |

以下两个接口涉及与收藏夹的交叉操作，**推迟到 Phase 3 实现**：

| 操作           | 方法 | 官方路径                        |
|---------------|------|--------------------------------|
| 复制到收藏夹   | POST | `/x/v2/history/toview/copy`    |
| 移动到收藏夹   | POST | `/x/v2/history/toview/move`    |

连播接口 `/x/v2/medialist/resource/list` 在 `type=0` 时用于稍后再看，
需要在客户端做**条件分流**（仅 watchLater 场景走自建服务器）。

#### 1.2 数据库表设计

```sql
CREATE TABLE watch_later (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL,           -- 本地用户 ID
    aid         INTEGER NOT NULL,           -- 视频 aid
    bvid        TEXT NOT NULL,              -- 视频 bvid
    title       TEXT,                       -- 视频标题
    pic         TEXT,                       -- 封面 URL
    duration    INTEGER,                    -- 时长(秒)
    owner_mid   INTEGER,                    -- UP 主 mid
    owner_name  TEXT,                       -- UP 主名称
    owner_face  TEXT,                       -- UP 主头像
    videos      INTEGER,                    -- 分P数
    cid         INTEGER,                    -- 默认 cid
    progress    INTEGER DEFAULT 0,          -- 观看进度(秒)
    pubdate     INTEGER,                    -- 发布时间戳
    bvid_meta   TEXT,                       -- 完整元数据 JSON(备用)
    added_at    INTEGER NOT NULL,           -- 添加时间戳
    viewed      INTEGER DEFAULT 0,          -- 0:未看 1:已看
    UNIQUE(user_id, aid)
);
CREATE INDEX idx_wl_user ON watch_later(user_id, added_at);
```

#### 1.3 服务端接口实现

**GET `/x/v2/history/toview/web`**

查询参数：`pn`, `ps`, `viewed`, `key`, `asc`

返回格式（与官方一致）：
```json
{
  "code": 0,
  "data": {
    "count": 42,
    "list": [
      {
        "aid": 123456,
        "bvid": "BV1xx...",
        "title": "视频标题",
        "pic": "https://i0.hdslb.com/...",
        "duration": 360,
        "pubdate": 1700000000,
        "cid": 789,
        "progress": 120,
        "videos": 1,
        "owner": { "mid": 100, "name": "UP主", "face": "..." },
        "stat": { "view": 0, "danmaku": 0 },
        "pages": [{ "cid": 789, "page": 1, "part": "P1" }],
        "is_pgc": false,
        "bvid": "BV1xx..."
      }
    ]
  }
}
```

注意：`stat`（播放量/弹幕数等）字段在自建服务器上无法实时获取，
可返回 0 或缓存值。客户端显示上这些数据是次要信息，不影响核心功能。

**POST `/x/v2/history/toview/add`**

请求体：`aid` 或 `bvid`（与官方完全一致）

服务端逻辑：
1. 检查是否已存在（UNIQUE 约束），已存在则更新 `added_at`
2. 向 B 站公开 API 查询视频元信息（`/x/web-interface/view`），填充 title/pic/owner 等字段
3. 已缓存过的视频可直接复用元数据，无需重复查询

**POST `/x/v2/history/toview/v2/dels`**

请求体：`resources`（逗号分隔的 aid 列表）

服务端逻辑：`DELETE FROM watch_later WHERE user_id = ? AND aid IN (?)`

**POST `/x/v2/history/toview/clear`**

请求体：`clean_type`（null=全部, 1=失效, 2=已看）

服务端逻辑：
- null → 删除该用户全部记录
- 1 → 自建服务器无"失效"概念，可跳过或不操作
- 2 → `DELETE FROM watch_later WHERE user_id = ? AND viewed = 1`

#### 1.4 客户端改动清单

| 文件                                    | 改动                                                     |
|-----------------------------------------|---------------------------------------------------------|
| `lib/http/api.dart:236`                 | `seeYouLater` 路径加 `selfBaseUrl` 前缀                   |
| `lib/http/api.dart:390`                 | `toViewLater` 路径加 `selfBaseUrl` 前缀                   |
| `lib/http/api.dart:393`                 | `toViewDel` 路径加 `selfBaseUrl` 前缀                     |
| `lib/http/api.dart:396`                 | `toViewClear` 路径加 `selfBaseUrl` 前缀                   |
| `lib/http/user.dart:64`                 | `seeYouLater()` 改用 SelfRequest，去掉 WbiSign            |
| `lib/http/user.dart:173`               | `toViewLater()` 改用 SelfRequest，去掉 csrf               |
| `lib/http/user.dart:197`               | `toViewDel()` 改用 SelfRequest，去掉 csrf                 |
| `lib/http/user.dart:232`               | `toViewClear()` 改用 SelfRequest，去掉 csrf               |
| `lib/http/user.dart:350`               | `getMediaList()` 当 type==0 时分流到 SelfRequest           |
| `lib/common/widgets/video_popup_menu.dart:65` | 无需改动（调用的是 UserHttp.toViewLater）             |

#### 1.5 元数据获取（服务端代理查询）

添加稍后再看时，官方 API 只需要 `aid/bvid`，B 站服务端会自动填充视频元数据。
自建服务器采用相同策略：**收到 aid/bvid 后，由服务端向 B 站 API 查询视频信息并缓存**。

服务端逻辑：
1. 收到 `add` 请求，提取 `aid` 或 `bvid`
2. 调用 B 站公开 API（如 `/x/web-interface/view?bvid=xxx`）获取视频元信息
3. 提取 title, pic, duration, owner, cid, pages 等字段写入数据库
4. 对已缓存的视频元数据可直接复用，无需重复查询

这样客户端代码**无需任何改动**，请求参数与官方完全一致。

注意事项：
- B 站视频信息查询接口是公开的，不需要登录态
- 服务端应缓存查询结果，避免频繁请求 B 站
- 如果视频已失效（被删除/下架），查询会返回错误，服务端仍应记录 aid/bvid，
  将 title 设为"已失效视频"等占位文本

---

### Phase 2：观看历史（Watch History）

**目标**：迁移观看历史和进度上报，这是隐私保护收益最大的部分。

#### 2.1 需要复制的官方 API

| 操作             | 方法 | 官方路径                              | 请求参数                          |
|-----------------|------|--------------------------------------|-----------------------------------|
| 获取历史列表     | GET  | `/x/web-interface/history/cursor`    | `type, ps(20), max, view_at`     |
| 搜索历史         | GET  | `/x/web-interface/history/search`    | `pn, keyword, business`          |
| 上报播放进度     | POST | `/x/click-interface/web/heartbeat`   | `bvid/aid, cid, played_time, ...`|
| 历史上报         | POST | `/x/v2/history/report`               | `aid, type`                      |
| 删除单条历史     | POST | `/x/v2/history/delete`               | `kid`                            |
| 清空历史         | POST | `/x/v2/history/clear`                |                                   |
| 暂停历史记录     | POST | `/x/v2/history/shadow/set`           | `switch`                         |
| 查询暂停状态     | GET  | `/x/v2/history/shadow?jsonp=jsonp`   |                                   |

#### 2.2 数据库表设计

```sql
CREATE TABLE watch_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL,
    aid         INTEGER NOT NULL,
    bvid        TEXT,
    cid         INTEGER,
    epid        INTEGER,                    -- 番剧剧集ID
    season_id   INTEGER,                    -- 番剧季ID
    title       TEXT,
    long_title  TEXT,
    cover       TEXT,
    duration    INTEGER,
    progress    INTEGER DEFAULT 0,          -- 最新进度(秒), -1=看完
    author_mid  INTEGER,
    author_name TEXT,
    author_face TEXT,
    badge       TEXT,                       -- "番剧"/"电影" 等标签
    kid         TEXT,                       -- 用于删除的 key (business_oid 格式)
    business    TEXT,                       -- archive/pgc/live/article 等
    view_at     INTEGER NOT NULL,           -- 最近观看时间戳
    videos      INTEGER,                    -- 分P总数
    current     TEXT,                       -- 当前观看分P名
    is_finish   INTEGER DEFAULT 0,
    is_fav      INTEGER DEFAULT 0,
    meta_json   TEXT,                       -- 完整元数据备用
    UNIQUE(user_id, aid)
);
CREATE INDEX idx_hist_user_time ON watch_history(user_id, view_at DESC);
CREATE INDEX idx_hist_search ON watch_history(user_id, title);

-- 历史记录暂停状态
CREATE TABLE user_settings (
    user_id         INTEGER PRIMARY KEY,
    history_paused  INTEGER DEFAULT 0       -- 0:正常记录 1:暂停
);
```

#### 2.3 Heartbeat 上报（关键机制）

播放器每 5 秒调用一次 heartbeat，上报当前进度。这是最高频的请求。

服务端实现：
```
POST /x/click-interface/web/heartbeat
Body: { bvid, cid, played_time, epid?, sid?, type, sub_type? }
```
- 收到后 `UPSERT` watch_history 表，更新 `progress` 和 `view_at`
- 如果该视频不在历史中，创建新记录，同时向 B 站查询视频元信息并填充（同 Phase 1 的代理查询策略）

**性能考量**：heartbeat 是高频请求（每 5 秒一次），服务端应：
- 使用批量写入或写缓冲
- 或者客户端降低上报频率（如改为 15-30 秒）

#### 2.4 游标分页

官方历史列表使用**游标分页**而非页码分页：
- 请求参数：`max`（上一页最后一条的 oid）、`view_at`（上一页最后一条的时间戳）
- 响应中 `cursor` 字段携带下一页的 `max` 和 `view_at`

自建服务器需要实现相同的游标逻辑。

#### 2.5 客户端改动清单

| 文件                                          | 改动                                               |
|----------------------------------------------|----------------------------------------------------|
| `lib/http/api.dart:239`                      | `historyList` 路径分流到 selfBaseUrl                  |
| `lib/http/api.dart:248-254`                  | `clearHistory`, `delHistory`, `searchHistory` 分流   |
| `lib/http/api.dart:274`                      | `heartBeat` 路径分流到 selfBaseUrl                    |
| `lib/http/api.dart:276`                      | `historyReport` 路径分流                              |
| `lib/http/user.dart:84-105`                  | `historyList()` 改用 SelfRequest                     |
| `lib/http/user.dart:108-165`                 | `pauseHistory()`, `historyStatus()`, `clearHistory()` 改用 SelfRequest |
| `lib/http/user.dart:248-270`                 | `delHistory()` 改用 SelfRequest                      |
| `lib/http/user.dart:287-306`                 | `searchHistory()` 改用 SelfRequest                   |
| `lib/http/video.dart:657-715`                | `historyReport()`, `heartBeat()`, `medialistHistory()` 改用 SelfRequest |
| `lib/plugin/pl_player/controller.dart:1721`  | `makeHeartBeat()` 内部调用已走 VideoHttp，无需直接改动   |

#### 2.6 多账号历史（注意点）

当前代码中 `historyList()` 和 `delHistory()` 使用 `Accounts.history`（可能与主账号不同），
heartbeat 使用 `Accounts.heartbeat`。迁移后统一由 JWT 标识用户即可，
但需注意客户端原有的多账号逻辑在自建服务器模式下如何处理。

---

### Phase 3：收藏夹（Favorites）

**目标**：迁移最复杂的模块——用户收藏体系。

#### 3.1 需要复制的官方 API

##### 收藏夹管理

| 操作           | 方法 | 官方路径                                | 说明                   |
|---------------|------|-----------------------------------------|------------------------|
| 获取全部文件夹  | GET  | `/x/v3/fav/folder/created/list-all`    | 同时可查视频属于哪些夹   |
| 获取文件夹列表  | GET  | `/x/v3/fav/folder/created/list`        | 分页                    |
| 获取文件夹详情  | GET  | `/x/v3/fav/folder/info`               |                        |
| 创建文件夹     | POST | `/x/v3/fav/folder/add`                |                        |
| 编辑文件夹     | POST | `/x/v3/fav/folder/edit`               |                        |
| 删除文件夹     | POST | `/x/v3/fav/folder/del`                |                        |
| 排序文件夹     | POST | `/x/v3/fav/folder/sort`               | 需 AppSign              |

##### 收藏内容管理

| 操作           | 方法 | 官方路径                            | 说明                         |
|---------------|------|-------------------------------------|------------------------------|
| 获取文件夹内容  | GET  | `/x/v3/fav/resource/list`          | 分页，可搜索、排序             |
| 批量收藏/取消   | POST | `/x/v3/fav/resource/batch-deal`    | `add_media_ids, del_media_ids`|
| 取消全部收藏    | POST | `/x/v3/fav/resource/unfav-all`     |                              |
| 复制收藏       | POST | `/x/v3/fav/resource/copy`          |                              |
| 移动收藏       | POST | `/x/v3/fav/resource/move`          |                              |
| 清理失效内容    | POST | `/x/v3/fav/resource/clean`         |                              |
| 排序收藏       | POST | `/x/v3/fav/resource/sort`          | 需 AppSign                    |

##### 与稍后再看的交叉操作（此时实现）

| 操作               | 方法 | 官方路径                         |
|-------------------|------|----------------------------------|
| 稍后再看→复制到收藏夹 | POST | `/x/v2/history/toview/copy`     |
| 稍后再看→移动到收藏夹 | POST | `/x/v2/history/toview/move`     |

#### 3.2 数据库表设计

```sql
-- 收藏夹
CREATE TABLE fav_folder (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL,
    media_id    INTEGER NOT NULL,           -- 对外暴露的文件夹 ID
    title       TEXT NOT NULL,
    cover       TEXT DEFAULT '',
    intro       TEXT DEFAULT '',
    privacy     INTEGER DEFAULT 0,          -- 0:公开 1:私密（自建服务器中均为私密）
    media_count INTEGER DEFAULT 0,
    ctime       INTEGER,
    mtime       INTEGER,
    sort_order  INTEGER DEFAULT 0,          -- 排序权重
    is_default  INTEGER DEFAULT 0,          -- 是否默认收藏夹
    UNIQUE(user_id, media_id)
);

-- 收藏内容
CREATE TABLE fav_resource (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL,
    media_id    INTEGER NOT NULL,           -- 所属文件夹
    resource_id INTEGER NOT NULL,           -- 视频 aid
    resource_type INTEGER DEFAULT 2,        -- 2:视频
    title       TEXT,
    cover       TEXT,
    intro       TEXT,
    duration    INTEGER,
    upper_mid   INTEGER,
    upper_name  TEXT,
    bvid        TEXT,
    pubtime     INTEGER,
    fav_time    INTEGER NOT NULL,           -- 收藏时间
    meta_json   TEXT,                       -- 完整元数据备用
    UNIQUE(user_id, media_id, resource_id)
);
CREATE INDEX idx_fav_res ON fav_resource(user_id, media_id, fav_time DESC);
```

#### 3.3 客户端改动清单

涉及文件较多，核心改动集中在：

| 文件                              | 改动                                            |
|-----------------------------------|-------------------------------------------------|
| `lib/http/api.dart` 约 15 个端点   | 收藏相关路径全部加 selfBaseUrl 前缀               |
| `lib/http/fav.dart` 全文           | 所有方法改用 SelfRequest，移除 csrf 和 AppSign    |
| `lib/http/user.dart:309-328`      | `userSubFolder()` 如果仅含收藏夹订阅则分流         |

#### 3.4 不迁移的收藏子类型（建议）

以下收藏子类型与 B 站内容系统深度耦合，初期可不迁移：

| 子类型   | 原因                                         |
|---------|----------------------------------------------|
| 专栏收藏 | 文章/动态内容需 B 站 ID 体系                    |
| 课堂收藏 | PUGV（付费课程）依赖 B 站支付体系                |
| 话题收藏 | 话题是 B 站平台概念                             |
| 笔记     | 笔记关联到具体视频时间线，结构复杂                |

初期只迁移**视频收藏夹**（最核心、数据量最大的部分）。

---

### Phase 4：关注 / 追番 / 订阅

**目标**：迁移用户的内容订阅关系。

#### 4.1 迁移范围与取舍

| 功能             | 是否迁移 | 说明                                            |
|-----------------|---------|------------------------------------------------|
| 关注列表         | **是**   | 迁移后作为本地订阅列表，不再通知 UP 主              |
| 关注分组/标签     | **是**   | 纯用户侧数据                                    |
| 特别关注         | **是**   | 纯用户侧数据                                    |
| 粉丝列表         | **否**   | 这是别人对你的关注，数据不在你手中                  |
| 追番/追剧        | **是**   | 迁移后变为本地追踪列表                             |
| 追番状态(想看/在看/已看) | **是** | 纯用户侧标记                               |

关注操作完全脱离官方 API，不再向 B 站发送关注请求。
迁移后关注本质上等同于本地 RSS 订阅：用于内容追踪和 UI 展示。

#### 4.2 需要复制的官方 API

##### 关注管理

| 操作           | 方法 | 官方路径                          |
|---------------|------|-----------------------------------|
| 获取关注列表   | GET  | `/x/relation/followings`          |
| 搜索关注       | GET  | `/x/relation/followings/search`   |
| 获取关注分组   | GET  | `/x/relation/tags`                |
| 获取分组成员   | GET  | `/x/relation/tag`                 |
| 创建分组       | POST | `/x/relation/tag/create`          |
| 修改分组名     | POST | `/x/relation/tag/update`          |
| 删除分组       | POST | `/x/relation/tag/del`             |
| 添加到分组     | POST | `/x/relation/tags/addUsers`       |
| 特别关注-添加  | POST | `/x/relation/tag/special/add`     |
| 特别关注-移除  | POST | `/x/relation/tag/special/del`     |

##### 追番/追剧

| 操作           | 方法 | 官方路径                              |
|---------------|------|---------------------------------------|
| 获取追番列表   | GET  | `/x/space/bangumi/follow/list`        |
| 追番           | POST | `/pgc/web/follow/add`                |
| 取消追番       | POST | `/pgc/web/follow/del`                |
| 更新追番状态   | POST | `/pgc/web/follow/status/update`      |

#### 4.3 数据库表设计

```sql
-- 关注列表
CREATE TABLE following (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL,           -- 本地用户
    mid         INTEGER NOT NULL,           -- 被关注者的 B 站 mid
    uname       TEXT,
    face        TEXT,
    sign        TEXT,                       -- 签名
    special     INTEGER DEFAULT 0,          -- 1:特别关注
    mtime       INTEGER,                    -- 关注时间
    UNIQUE(user_id, mid)
);

-- 关注分组
CREATE TABLE follow_tag (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL,
    tagid       INTEGER NOT NULL,           -- 分组 ID
    name        TEXT NOT NULL,
    UNIQUE(user_id, tagid)
);

-- 分组-用户关联
CREATE TABLE follow_tag_member (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL,
    tagid       INTEGER NOT NULL,
    mid         INTEGER NOT NULL,           -- 被关注者
    UNIQUE(user_id, tagid, mid)
);

-- 追番/追剧
CREATE TABLE bangumi_follow (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL,
    season_id       INTEGER NOT NULL,
    media_id        INTEGER,
    title           TEXT,
    cover           TEXT,
    season_type     INTEGER,                -- 1:番剧 2:电影 3:纪录片 ...
    total_count     INTEGER,
    is_finish       INTEGER DEFAULT 0,
    follow_status   INTEGER DEFAULT 0,      -- 0:未标记 1:想看 2:在看 3:已看
    new_ep_desc     TEXT,                   -- 最新集描述
    progress        TEXT,                   -- 观看进度描述
    mtime           INTEGER,
    meta_json       TEXT,
    UNIQUE(user_id, season_id)
);
```

#### 4.4 客户端改动清单

| 文件                                   | 改动                                          |
|----------------------------------------|-----------------------------------------------|
| `lib/http/api.dart` 约 12 个端点        | 关注/追番相关路径分流到 selfBaseUrl              |
| `lib/http/follow.dart`                 | `followings()` 改用 SelfRequest                |
| `lib/http/member.dart:472-620`         | 分组相关方法改用 SelfRequest                    |
| `lib/http/video.dart:594-640`          | `relationMod()` 完全走 SelfRequest                       |
| `lib/http/video.dart:718-771`          | `pgcAdd()`, `pgcDel()`, `pgcUpdate()` 走 SelfRequest |
| `lib/http/fav.dart:375-395`            | `favPgc()` 改用 SelfRequest                    |
| `lib/http/dynamics.dart:74-87`         | `followUp()` 改用 SelfRequest                   |
| `lib/utils/request_utils.dart:101-241` | `actionRelationMod()` 完全走 SelfRequest         |

---

## 五、实施优先级与依赖关系

```
Phase 0 (基础设施)
    │
    ├── Phase 1 (稍后再看) ← 最先实施，验证方案
    │
    ├── Phase 2 (观看历史) ← 隐私收益最大
    │       │
    │       └── heartbeat 进度同步
    │
    ├── Phase 3 (收藏夹) ← 最复杂，但结构清晰
    │       │
    │       └── 与 Phase 1 的交叉操作 (copy/move toview)
    │
    └── Phase 4 (关注/订阅)
```

建议实施顺序：**Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 4**

每个 Phase 完成后应能独立运行，未迁移的功能继续走官方 API。

---

## 六、关键风险与应对

| 风险                              | 影响     | 应对措施                                          |
|-----------------------------------|---------|--------------------------------------------------|
| 视频元数据获取                     | Phase 1-3| 服务端代理查询 B 站公开 API 并缓存                   |
| heartbeat 高频写入性能             | Phase 2  | 写缓冲 + 降低上报频率                               |
| 收藏夹 AppSign 签名               | Phase 3  | 自建服务器无需验签，客户端去掉 AppSign 调用           |
| 关注列表与动态流的关联              | Phase 4  | 动态流仍从 B 站获取，关注列表仅用于 UI 展示和筛选      |
| 离线/网络不可用                    | 全局     | 客户端本地 SQLite 缓存 + 上线后同步                  |

---

## 七、目录结构（建议）

```
PiliMinusB/
├── PLAN.md                  ← 本文件
├── server/                  ← 自建服务器
│   ├── main.go (或 index.ts)
│   ├── config/
│   ├── middleware/
│   │   └── auth.go          ← JWT 鉴权中间件
│   ├── handler/
│   │   ├── auth.go          ← 注册/登录
│   │   ├── watch_later.go   ← Phase 1
│   │   ├── history.go       ← Phase 2
│   │   ├── favorite.go      ← Phase 3
│   │   └── follow.go        ← Phase 4
│   ├── model/
│   ├── database/
│   │   └── schema.sql
│   └── go.mod (或 package.json)
└── client-patches/          ← 客户端改动补丁/说明
    ├── phase0-infra.md
    ├── phase1-watch-later.md
    ├── phase2-history.md
    ├── phase3-favorite.md
    └── phase4-follow.md
```
