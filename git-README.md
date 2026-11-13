# FRP-Panel 构建与发布指南

本项目已对 Windows 环境下的 `workerd` 服务支持进行了兼容适配，并配置了完善的 GitHub Actions 跨平台持续集成与自动发布流水线。

---

## 一、GitHub Actions 自动构建与发布机制

仓库已配置好全自动云端编译与发布，**日常开发与发布无需在本地手动交叉编译**。

### 1. 正式版本发布（推荐）
当推送符合 `v*` 规则的 Git Tag 时，会触发 `.github/workflows/tag.workflow.yml`：
- **云端动作**：自动拉取代码、构建前端静态页面（Next.js）、交叉编译所有平台的独立二进制包。
- **发布产物**：
  - 自动创建并发布对应的 GitHub Release（如 `v0.1.39`）。
  - 自动同步覆盖更新 `latest` 发行版。

**操作命令**：
```bash
# 1. 提交代码
git add .
git commit -m "feat: 你的更新说明"
git push origin main

# 2. 打标签并推送到 GitHub
git tag v0.1.39
git push origin v0.1.39
```

---

### 2. 日常代码提交（主分支触发）
当直接向 `main` 分支推送代码（`git push origin main`）时，会触发 `.github/workflows/master.workflow.yml`：
- **云端动作**：自动构建全平台二进制包。
- **发布产物**：自动生成/更新 GitHub 上的 **`alpha`** 预发布版本（Pre-release）。

---

### 3. GitHub 网页手动一键构建（Workflow Dispatch）
若需要临时重新构建全平台包，无需提交新代码：
1. 打开 GitHub 仓库页面，点击顶部的 **Actions** 标签。
2. 在左侧选择 **Master Release** 或 **Version Tag Release**。
3. 点击右侧的 **Run workflow** 下拉框，选择分支并点击按钮运行。

---

## 二、支持的平台与二进制文件清单

每次发布会自动编译以下 18 个独立的平台二进制包（包含服务端与客户端）：

| 操作系统 | 架构 | 文件名 | 说明 |
| :--- | :--- | :--- | :--- |
| **Windows** | x86_64 / amd64 | `frp-panel-windows-amd64.exe`<br>`frp-panel-client-windows-amd64.exe` | **支持 Windows 原生 workerd** |
| **Windows** | arm64 | `frp-panel-windows-arm64.exe`<br>`frp-panel-client-windows-arm64.exe` | Windows ARM 设备使用 |
| **Linux** | x86_64 / amd64 | `frp-panel-linux-amd64`<br>`frp-panel-client-linux-amd64` | 常用 Linux 服务器 |
| **Linux** | aarch64 / arm64 | `frp-panel-linux-arm64`<br>`frp-panel-client-linux-arm64` | 树莓派 4/5、ARM 云服务器 |
| **Linux** | ARMv7 / ARMv6 | `frp-panel-linux-armv7l` / `armv6l`<br>`frp-panel-client-linux-armv7l` / `armv6l` | 嵌入式与老旧 ARM 设备 |
| **Linux** | RISC-V 64 | `frp-panel-linux-riscv64`<br>`frp-panel-client-linux-riscv64` | RISC-V 架构设备 |
| **macOS** | Intel (amd64) | `frp-panel-darwin-amd64`<br>`frp-panel-client-darwin-amd64` | Intel Mac |
| **macOS** | Apple Silicon | `frp-panel-darwin-arm64`<br>`frp-panel-client-darwin-arm64` | M1/M2/M3/M4 系列 Mac |

- `frp-panel-*`：完整版，包含服务端功能以及内嵌的 Web 管理面板。
- `frp-panel-client-*`：纯客户端，用于节点接入。

---

## 三、本地编译指南

仅在本地修改代码需要**立即在当前机器进行调试与功能验证**时，才建议在本地执行构建。

### 1. 前端构建（主程序 Web 控制台）
主程序 `cmd/frpp` 内嵌了 `cmd/frpp/out` 静态资源。若修改了前端页面，需重新构建：
```bash
cd www
pnpm install
pnpm build
cd ..
```
*构建完成后会自动将导出文件同步至 `cmd/frpp/out` 目录。*

### 2. 本地快速编译可执行文件
在项目根目录下执行：

- **Windows 本地构建**：
  ```powershell
  # 编译完整主端 (带控制台)
  go build -o frp-panel.exe ./cmd/frpp

  # 编译纯客户端
  go build -o frp-panel-client.exe ./cmd/frppc
  ```

- **Linux / macOS 本地构建**：
  ```bash
  # 编译完整主端
  go build -o frp-panel ./cmd/frpp

  # 编译纯客户端
  go build -o frp-panel-client ./cmd/frppc
  ```

---

## 四、Windows 平台 Workerd 运行说明

本分支已补齐 Windows 平台对 Cloudflare `workerd` 的兼容支持：
1. **自动路径检索**：优先从系统 PATH、当前目录、执行文件所在目录以及标准安装目录（如 `D:\Program Files\frpp\workerd.exe`）查找 `workerd.exe`。
2. **端口与网络兼容**：由于 Windows 原生不支持 Linux 的 `unix-abstract` 套接字，本分支已实现自动映射为本地回环 TCP 端口（12000-32000 范围哈希映射），无缝对接 FRP 隧道代理。
3. **进程生命周期与资源清理**：实现了 Windows 下的平滑停止信号与进程句柄释放重试，避免文件占用冲突（Sharing Violation）。

**客户端运行示例**：
```powershell
.\frp-panel-client-windows-amd64.exe client `
  -s <CLIENT_SECRET> `
  -i <CLIENT_ID> `
  --api-url https://<PANEL_HOST>:443 `
  --rpc-url wss://<PANEL_HOST>:443
```
