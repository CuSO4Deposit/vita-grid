# vita-grid

LED 状态显示面板：服务器聚合状态 → HTTP → ESP32 → WS2812 8x8 矩阵。
用户自己写代码。详见 PROGRESS.md 的当前进度与下一步。

## 架构

```
每台机器:   statusd（本地状态守护进程，各机器配置不同，契约相同）
                ↓ HTTP（内网/隧道，端口 8081 约定）
一台机器:   聚合器（灯 query 的主服务）
             抓 statusd + Upptime + web → 合并 → 缓存 → /status + /config（端口 8080）
                ↓ HTTP（公网，供家里+公司两块灯板）
            ESP32 ×2 轮询 /status + /config → WS2812 8x8
```

## 契约（已锁定，改动需讨论）

### /status 输出格式

```json
[{"name":"laborari:docker","state":"ok","busy":false,"extra":...}, ...]
```

- `name` = **可读名**：upptime 站点名（聚合器 config `sites[].name` 透传，缺省 = upptime 文件名，无组/无前缀）/ `web-alive` / 机器信号带 host 前缀（`laborari:docker`，防跨机重名）。不搞代号层（域名/机器名本就公开）
- `state` = 三态：`ok` / `warn` / `error`
- `busy` = 正交布尔，true 表示"正在做某事"→ 显示端闪烁
- 服务器端归一化：答不上来/数据过期一律 `warn`；**例外：整台机器/statusd 不可达 = `error`**（"确定知道坏了"=红，"不知道坏没坏"=黄）

### 板端配置与显示映射（聚合器 /config 下发）

固件编译期只留"启动必需"：`wifi`、聚合器 base URL（`/status` 与 `/config/<board>` 共用）、板子 `board` id、`led.order`（GRB，FastLED 模板参数，WS2812B 固定）。

其余全部由聚合器下发：聚合器 `-boards <dir>` 指向板端 JSON 目录，**启动时读入内存**，挂到 `GET /config/<board>`（改文件后需重启聚合器生效；聚合器不做解析/校验）。目录内容由 NixOS 模块声明式组装：`aggregator.boards`（`<board-id>` → **内联 Nix JSON 数据**，`builtins.toJSON` 序列化）生成 `boardsDir/<board>.json`，`-boards` 指向该目录。

```json
{
  "version": 1,
  "poll": { "interval": 30 },
  "led": { "pin": 4, "count": 64, "serpentine": true },
  "bh1750": { "enabled": true, "sda": 21, "scl": 22, "offLux": 20 },
  "brightness": { "level": 50 },
  "colors": {
    "ok": [0, 255, 0],
    "warn": [255, 255, 0],
    "error": [255, 0, 0],
    "absent": [255, 0, 0]
  },
  "blink": { "intervalMs": 500 },
  "mapping": [{ "name": "host:service", "index": 0 }]
}
```

- 改 `boardsDir/<board>.json` → 重启聚合器 → 板子下次拉取生效，不用重烧；板端 config 全公开（无 wifi 等机密），不走 agenix
- 显示映射：`ok`→`colors.ok` / `warn`→`colors.warn` / `error`→`colors.error`；`busy=true` → 按该色闪烁；缺席信号 → `colors.absent`
- 聚合器不校验板端 config（纯文件服务）；固件兜底：mapping 空/越界 index 丢弃并记日志、config 拉取失败用 NVS 存的最后一份、从未成功 → 红色闪烁报错

### statusd 契约

- 输出同一份 JSON 列表，statusd 内部用**本地可读名**（不带前缀；前缀由聚合器按 host 加上）
- 每信号声明 `kind`：`systemd-active`（查常驻服务 `is-active`）/ `systemd-result`（查 oneshot/timer 触发的服务上次运行结果，用 `Result`，**勿用 is-active**；时效看 `timer` 单元的 `LastTriggerUSec` + `maxAge`，**勿用服务时间戳**——systemd 会清空）/ `marker`（读标记文件，暂缓）/ `cmd`（跑命令看退出码）/ `tcp`（连端口）
- kind 拆分规则：命令/解析/映射全不同 → 独立 kind；只差小参数 → 同 kind + 字段
- 端口约定 8081

### 聚合器配置字段

- `listen`、`sources[]`（upptime / statusd / web）、每源 `refresh` 秒；板端 JSON 目录经 CLI flag `-boards <dir>` 传入（→ `GET /config/<board>` 文件服务）

## Upptime 数据源（已确认）

- 仓库：`CuSO4Deposit/literate-journey`（public，default branch: master）
- 数据：`raw.githubusercontent.com/<repo>/master/history/<file>.yml`（每站点单文件，含最新 `status: up/down`），免认证不限流
- `.upptimerc.yml` = 站点清单（name→URL）；API 文件每 5 分钟更新 → 刷新周期 300s
- 文件名：**不做 slugify**，聚合器 config `sites[].file` 透传 upptime 真实文件名（site 改名后 history 文件名不变，slug 推导必然脆弱；`rss-hub` 就是 `rss-hub`）
- 状态推导：per-site 读 `history/<file>.yml` 的 `status` 字段；**单信号平铺，无 host 分组、无 worst-of 组头**（分组由板端 mapping 排）
- 别碰 `api.github.com`（60 次/小时限流）；status.depoze.xyz 上无 api 数据（SPA 产物）

## 技术栈

- statusd + 聚合器：**Go**（标准库 net/http / encoding/json），编译单静态二进制，跨机部署=拷一个文件；
- 本地验证：`nix shell nixpkgs#go -c go build/run`
- ESP32 固件：Arduino C++（NeoPixel 库 + 轮询 + JSON 解析 + BH1750）
- NixOS flake：公开模块（statusd/聚合器 systemd 服务），私有配置走 agenix

## 关键决策记录

- 探测方式：**自报式 statusd**（非 SSH 侵入）；机器开机 = 聚合器能否抓到其 statusd
- 局域网：statusd 绑 LAN + 聚合机 nginx 路径分流；远程机走隧道（如 WireGuard）
- 备份状态源：待定（用户多机方案未统一），statusd 里以 `marker` kind 预留
- 隐私边界：域名/hostname 均已公开；`/status` 用可读名输出；暂不做鉴权（可后加 token）
- 板端配置从编译期搬到聚合器 `/config` 下发：改 `boardsDir/<board>.json` + 重启聚合器即换灯，无需重烧；聚合器只做文件服务（启动读入内存，零解析/校验）；固件只留 wifi + baseURL + board id + led.order
- 板端 config 注入：NixOS 模块 `aggregator.boards`（`<board-id>` → 内联 Nix JSON 数据，`builtins.toJSON` 序列化）声明式组装目录 → `-boards` flag 传入服务；板端 config 全公开，不走 agenix
- Upptime 站点名**不再派生文件名**（去掉 slugify，`RSSHub`→`rsshub` 曾致 404）：`sites[].file` 透传真实文件名；`sites[].name` 缺省=file；per-site 平铺信号，去掉 host 分组与 worst-of 组头（分组由板端 mapping 排）
- 用户非 NixOS 协作者不存在；本仓库是模块库，被用户的公开 nixos config 当 input 引入

## 验证命令

```bash
curl -s http://localhost:8080/status
```
