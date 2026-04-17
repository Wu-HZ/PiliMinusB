# sauc_go

`sauc_go` 基于火山 ASR demo 改造，当前已经可以：

- 用 `file` 模式识别本地音频文件
- 用 `http` 模式接收上传音频并返回字幕
- 用 `websocket` 模式接收实时音频分片并回推中间字幕
- 输出 JSON、纯文本和 SRT 字幕

## 目录说明

实际 Go 工程目录是当前这个 `sauc_go/sauc_go`，不是上一层外壳目录。

## 配置方式

方式 1：环境变量

```powershell
$env:VOLC_APP_KEY="your_app_key"
$env:VOLC_ACCESS_KEY="your_access_key"
```

方式 2：配置文件

在当前目录创建 `config.toml`：

```toml
[auth]
app_key = "your_app_key"
access_key = "your_access_key"
```

可参考 [config.toml.example](./config.toml.example)。

## 启动方式

### 文件模式

```powershell
go run . -mode file -file .\demo.wav -output srt
```

可选输出格式：

- `json`
- `text`
- `srt`

### HTTP 模式

```powershell
go run . -mode http -listen :8090
```

健康检查：

```powershell
curl.exe http://127.0.0.1:8090/healthz
```

上传音频：

```powershell
curl.exe -X POST "http://127.0.0.1:8090/transcribe" `
  -F "file=@D:\path\demo.wav"
```

直接返回 SRT：

```powershell
curl.exe -X POST "http://127.0.0.1:8090/transcribe?format=srt" `
  -F "file=@D:\path\demo.wav"
```

### 实时字幕 WebSocket

启动服务：

```powershell
go run . -mode http -listen :8090
```

实时字幕入口：

```text
ws://127.0.0.1:8090/realtime/ws?format=pcm&codec=raw&rate=16000&bits=16&channel=1
```

约定：

- 下游客户端发 `binary message`：每条消息是一段实时音频分片
- 推荐格式：`16kHz / 单声道 / 16bit PCM / raw`
- 客户端发 `{"type":"end"}`：表示音频结束，服务端会发送最后一包并等待最终字幕
- 客户端发 `{"type":"cancel"}`：立即取消上游识别
- 服务端会回推 `ready`、`partial`、`final`、`error` 四类消息

浏览器最小示例：

```javascript
const ws = new WebSocket(
  "ws://127.0.0.1:8090/realtime/ws?format=pcm&codec=raw&rate=16000&bits=16&channel=1"
);

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log(message.type, message.text, message.response);
};

ws.onopen = () => {
  // pcmChunk 需要是 Int16 PCM 对应的 ArrayBuffer/Uint8Array
  ws.send(pcmChunk);

  // 音频结束时显式发送 end，拿最终结果
  ws.send(JSON.stringify({ type: "end" }));
};

// 用户退出或不再需要字幕时
function cancelRealtime() {
  ws.send(JSON.stringify({ type: "cancel" }));
  ws.close();
}
```

## 跑通记录

这次把 demo 跑通，核心不是“只要能连上火山就行”，而是把几个协议和工程问题一起修正了。

### 1. 先把 demo 变成可接入服务

原始 demo 只有“读本地文件 -> 直接连 WebSocket -> 打日志”的入口，不适合业务接入。

现在增加了 HTTP 服务层：

- `POST /transcribe`：接收上传音频
- `GET /healthz`：健康检查
- `GET /realtime/ws`：实时字幕 WebSocket
- `?format=srt`：直接返回 SRT

### 2. 配置加载改成可落地方式

原始代码在 `init()` 里强依赖 `../config.toml`，没有文件就直接退出。

现在支持两种方式：

- 环境变量 `VOLC_APP_KEY` / `VOLC_ACCESS_KEY`
- 当前目录 `config.toml`

这样本地调试、容器部署、CI 都更容易接。

### 3. 去掉了不兼容的 `sonic`

原 demo 依赖 `github.com/bytedance/sonic`，在当前 Go 环境下会出现链接失败：

```text
invalid reference to encoding/json.unquoteBytes
```

这里直接改回标准库 `encoding/json`，避免编译兼容性问题。

### 4. 修正了 WebSocket 音频包 sequence

这里是第一个真正的协议坑。

表现：

- 服务端报 `autoAssignedSequence mismatch`

原因：

- `full request` 已经占用了 `seq=1`
- 第一个音频包不能再从 `1` 或 `-1` 开始
- 正确序列应该是 `2, 3, 4, ..., -N`

修正后，最后一包会发负序号，前面的音频包按正序递增。

### 5. 修正了音频载荷格式

这里是第二个协议坑。

表现：

- 服务端报 `Invalid audio format`
- 具体错误包含 `invalid WAV file format`

原因：

- 请求里声明的是 `format=wav`
- 但实际发的是去掉头部的裸 PCM

修正方式：

- 保留并发送完整 WAV 容器字节
- 同时仍然读取 WAV 头，用于推断采样率、声道、位深

### 6. 统一音频规范化策略

为了减少火山侧解码失败概率，现在会把输入音频规范化为：

- `16kHz`
- `单声道`
- `16bit PCM`

如果上传的不是标准 WAV，会调用本机 `ffmpeg` 转换。所以本机如果要接 mp3、m4a、aac 等格式，必须保证 `ffmpeg` 在 `PATH` 里。

### 7. `bigmodel_nostream` 不能按实时速度慢慢推

这里是第三个最关键的坑。

表现：

- 最终能出结果
- 但整个请求耗时基本等于音频本身时长

原因：

- 客户端仍在用“实时流式”节奏，每 `200ms` 发一包
- 这会导致 3 分钟音频真的传 3 分钟

修正方式：

- 当 URL 对应 `bigmodel_nostream` 时，直接快速上传全部分片
- 只有流式模式才按 `segmentDuration` 节流发送

所以现在的总耗时应该接近：

- 文件上传耗时
- 火山服务处理耗时

而不是“音频原时长 + 处理耗时”。

### 8. 把请求超时真正接到 WebSocket 生命周期

现在 `context timeout` 到期后会主动关闭 stop channel 和 websocket，避免某些异常情况下请求一直挂住。

### 9. 增加了真正可取消的实时字幕链路

这次新增了一条和 `nostream` 分开的实时链路：

- 下游客户端通过 `/realtime/ws` 持续发送音频分片
- 服务端再把分片转发给火山流式接口 `bigmodel`
- 用户主动发 `cancel` 或直接断开连接时，会立刻关闭上游 websocket

这样实时场景下就不需要先把整段音频都推完，用户中途退出时，后面的字幕也不会继续生成。

## 当前实现约束

- 默认 WebSocket URL 是 `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream`
- 默认实时 WebSocket URL 是 `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel`
- `-nonstream auto` 会根据 URL 自动推断 `enable_nonstream`
- 当前 HTTP 接口只支持单文件上传
- `/transcribe` 返回最终识别结果
- `/realtime/ws` 返回中间字幕和最终字幕

## 常见问题

### 1. 上传后一直很慢

先确认是不是旧进程还在跑。每次改完代码后，必须重启 `go run`。

### 2. 报 `autoAssignedSequence mismatch`

说明 sequence 又偏了。优先检查音频包是否从 `2` 开始，最后一包是否为负数。

### 3. 报 `invalid WAV file format`

优先检查两件事：

- 发出去的是不是完整 WAV 字节
- 请求里声明的 `format` 是否与实际载荷一致

### 4. 非 wav 文件识别失败

先确认本机是否安装并配置了 `ffmpeg`。

### 5. 实时字幕没有最终结果

优先检查两件事：

- 客户端是否在音频结束时显式发送了 `{"type":"end"}`
- 实时音频分片是否为 `pcm/raw/16k/mono/16bit`

## 建议的后续开发方向

- 增加结构化请求日志，打印文件名、分片数、总耗时、火山返回码
- 增加单元测试，至少覆盖 sequence、SRT 生成、配置加载
- 给 HTTP 接口增加文件大小限制和更明确的错误码
- 给 `/realtime/ws` 增加鉴权、会话 ID 和更细的事件模型
- 视客户端格式补齐 opus/webm/aac 到 pcm 的实时转码链路
