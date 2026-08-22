# vita-grid

LED 状态显示面板：服务器聚合状态 → HTTP → ESP32 → WS2812 8x8 矩阵。
用户自己写代码。详见 PROGRESS.md 的当前进度与下一步。

## 架构

```
每台机器:   agent 服务（本地状态服务器，各机器配置不同，契约相同）
                ↓ HTTP（内网/隧道，端口 8081 约定）
一台机器:   聚合器（灯 query 的主服务）
             抓 agent + Upptime + web → 改名成代号 → 缓存 → /status（端口 8080）
                ↓ HTTP（公网，供家里+公司两块灯板）
            ESP32 ×2 轮询 /status → WS2812 8x8
```

## 契约（已锁定，改动需讨论）

### /status 输出格式

```json
[{"name":"01","state":"ok","busy":false,"extra":...}, ...]
```

- `name` = **不透明代号**（聚合器统一分配，源侧可用可读名）
- `state` = 三态：`ok` / `warn` / `error`
- `busy` = 正交布尔，true 表示"正在做某事"→ 显示端闪烁
- 服务器端归一化：答不上来/数据过期一律 `warn`

### 显示端映射（ESP32 配置）

```
ok   → 绿  常亮
warn → 黄  常亮
error→ 红  常亮
busy=true → 按上述颜色闪烁
```

### agent 契约

- 输出同一份 JSON 列表，agent 内部用**本地可读名**（不带前缀）
- 每信号声明 `kind`：`systemd`（查 unit）/ `marker`（读标记文件）/ `cmd`（跑命令看退出码）/ `tcp`（连端口）
- 端口约定 8081

### 聚合器配置字段

- `listen`、`sources[]`（upptime / agent / web）、每源 `refresh` 秒、`rename` 代号映射

## Upptime 数据源（已确认）

- 仓库：`CuSO4Deposit/literate-journey`（public，default branch: master）
- 数据：`raw.githubusercontent.com/<repo>/master/api/<站点名>/...`，免认证不限流
- `.upptimerc.yml` = 站点清单（name→URL）；API 文件每 5 分钟更新 → 刷新周期 300s
- 状态推导：per-site 序列最后一条 up/down；host 分组 = 组内最差
- 别碰 `api.github.com`（60 次/小时限流）；status.depoze.xyz 上无 api 数据（SPA 产物）

## 技术栈

- agent + 聚合器：**Go**（标准库 net/http / encoding/json），编译单静态二进制，跨机部署=拷一个文件；
- 本地验证：`nix shell nixpkgs#go -c go build/run`
- ESP32 固件：Arduino C++（NeoPixel 库 + 轮询 + JSON 解析 + BH1750）
- NixOS flake：公开模块（agent/聚合器 systemd 服务），私有配置走 agenix

## 关键决策记录

- 探测方式：**自报式 agent**（非 SSH 侵入）；机器开机 = 聚合器能否抓到其 agent
- 局域网：agent 绑 LAN + 聚合机 nginx 路径分流；远程机走隧道（如 WireGuard）
- 备份状态源：待定（用户多机方案未统一），agent 里以 `marker` kind 预留
- 隐私边界：域名/hostname 均已公开；`/status` 用代号输出；暂不做鉴权（可后加 token）
- 用户非 NixOS 协作者不存在；本仓库是模块库，被用户的公开 nixos config 当 input 引入

## 验证命令

```bash
curl -s http://localhost:8080/status
```
