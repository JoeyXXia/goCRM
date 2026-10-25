# Day 1 启动与复核

适用于 Windows PowerShell；在新机器安装 Go 1.26.5、Node 22.14.0、npm 10.9.0 和 MySQL Community 8.0.44。Go module 仍保留上游最低版本与 toolchain 声明，运行验证版本见 UPSTREAM。Docker 未安装；本日不依赖 Docker。Redis 在原始配置中关闭，多点登录也关闭，因此本次不启动 Redis。

## 1. 核对与安装

在项目根目录运行：

```powershell
git rev-parse v2.9.2-stable
# 应为 a6882210a80bb27e3aa5dff0b4c21aa4afe8988a
go version
node --version
npm.cmd --version
mysql --version
./scripts/prepare-local.ps1
```

脚本不覆盖已有配置；生成随机 JWT key，`server/config.local.yaml` 被忽略。`.env.example` 只记录配置项，GVA 不会自动读取 `.env` 或 MYSQL_* 环境变量。GVA 仅支持 `-c` / `GVA_CONFIG` 选择 YAML，前端 Vite 读取 `web/.env.development`。不要把数据库密码写入 tracked `server/config.yaml`。

## 2. 原始后端

```powershell
Set-Location server
go mod download
go mod verify
go build -mod=readonly -o dealflow-server.exe .
./dealflow-server.exe -c config.local.yaml
```

如果共享 Go 缓存校验失败，不要忽略错误。此次验证遇到全局缓存两个目录被修改，最终使用项目专用缓存重新下载、校验和构建：在以上 Go 命令前设置 `$env:GOMODCACHE = Join-Path (Split-Path (Get-Location) -Parent) '.local/go-mod'`。专用缓存不提交，也不删除其他项目的全局缓存。

未初始化时 MySQL db-name 为空，后端允许进入初始化状态。默认端口 8888；`GET /health` 返回 `"ok"`，`POST /init/checkdb` 返回 `data.needInit=true`。这只证明进程启动，不代表数据库已连接。后端监听上游默认的 `:8888`，本日仅本地验证，不作为公开服务运行。

## 3. 原始前端

另开终端：

```powershell
Set-Location web
npm.cmd ci --no-audit --no-fund
npm.cmd exec vite -- --host 127.0.0.1 --mode development
```

打开 http://127.0.0.1:8080 。Vite 通过 `/api` 转发至 `127.0.0.1:8888`。也可使用上游 `npm run dev`，它会打开官方文档和浏览器并监听全部网卡；源码与声明保持原样。

## 4. 专用 MySQL 与初始化

准备可连接的 MySQL 8.0.44 实例和专用开发凭据，允许初始化器创建 **`dealflow_crm_day1`** 空库；不得填写现有业务库。前端初始化页面选择 mysql，输入 host/port/user/password/dbName 与独立 adminPassword；API 对应 `POST /init/initdb`（字段以 `server/model/system/request/sys_init.go` 为准）。初始化写入上游表与假数据，并回写当前选中的 `config.local.yaml`，同时更新 JWT key。

初始化完成后再次 `POST /init/checkdb` 应返回 `needInit=false`。若进程未自动重载，停止后端再用同一 `-c config.local.yaml` 启动。使用 `admin` 与刚设置的密码，填写验证码登录；应成功获取菜单。凭据与 token 不写到文档或日志。

本次验证使用独立的 MySQL 临时数据目录、`127.0.0.1:13306`，未使用已有 MySQL 服务。Windows MySQL 对当前中文绝对路径启动失败，因此实际临时数据放在另一个可写的 ASCII 路径；本机路径记录在被忽略的 `.local/mysql-runtime-path.txt`。这是本机验证工件，不是项目部署配置。`.local/credentials.json` 仅保存本机随机验证凭据，不提交。原始配置的验证码默认保持开启；自动 API 冒烟时可在本地配置临时把 open-captcha 调到 1000，完成后还原 0 并重启，再通过浏览器验证码验证。

## 5. 验收与清理

```powershell
# 项目根目录；服务运行时
./scripts/smoke-baseline.ps1
Set-Location web
npm.cmd run build
# 回到项目根目录
git diff --check
git status --short
git diff --exit-code v2.9.2-stable -- server/go.mod server/go.sum LICENSE
git check-ignore server/config.local.yaml .local/credentials.json
```

前端 build 不是组件/E2E 测试；HTTP/API 冒烟不是浏览器验收。新机器照此启动尚需独立复核，不因为当前机器可运行就宣称已通过。

完成后用 Ctrl+C 停止本次前后端进程。临时 MySQL 通过 mysqladmin 的配置文件方式 shutdown 或 Ctrl+C 关闭，只针对本次实例；保留本地配置供复现，不删除系统服务或现有数据库。不要把本地凭据、运行日志和数据库数据加入 Git。

## 提交建议

1. `chore: establish pinned GVA baseline`：上游基线、工具版本、锁文件、本地配置脚本与忽略规则。
2. `docs: define SaaS scope and upstream attribution`：范围、来源、调用链、启动与实际验证进度。

Day 1 在 day1/baseline 分支完成准备。2026-10-25 用户明确要求将 Day 1 推送到 https://github.com/JoeyXXia/goCRM ，交付分支采用目标仓库默认分支 main，保留固定上游父提交历史。Day 2 仅在用户继续安排后开始。发布操作记录见 [Day 1 GitHub 交付](progress/2026-10-25-day1-github.md)。
