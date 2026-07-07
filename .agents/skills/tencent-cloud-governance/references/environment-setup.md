# 运行环境

本技能依赖三个命令行工具：`tccli`（腾讯云 CLI，资源与费用查询）、`coscli`（COS 对象存储）和 `jq`（JSON 结果解析）。统一使用 `python3 -m venv` 在 `~/.tccli/venv` 创建隔离环境：

- `tccli` 安装到 `~/.tccli/venv`。
- `coscli` 放到 `~/.tccli/venv/bin/coscli`。
- `jq` 放到 `~/.tccli/venv/bin/jq`。
- 所有 `tccli` / `coscli` 操作前，都先 `source ~/.tccli/venv/bin/activate`。

## 创建虚拟环境

```sh
python3 -m venv ~/.tccli/venv
source ~/.tccli/venv/bin/activate
python -m pip install --upgrade pip
python -m pip install --upgrade tccli
tccli --version 2>&1 | tr -d '\r'
```

说明：

- 不要把 `tccli` 安装到系统 Python，也不要依赖全局 `pipx run` / `uv tool run`。
- `tccli` 的 OAuth/STS 凭证仍会写入用户的 tccli 凭证目录，用于后续自动刷新。
- 如果 `~/.tccli/venv` 已存在，直接 `source ~/.tccli/venv/bin/activate` 后升级即可：

```sh
source ~/.tccli/venv/bin/activate
python -m pip install --upgrade tccli
```

## 安装 coscli

`coscli` 下载到虚拟环境的 `bin` 目录，并命名为 `coscli`，这样激活虚拟环境后可以直接运行 `coscli`，`scripts/coscli-auth.sh` 也能按默认值找到它。

先激活虚拟环境：

```sh
source ~/.tccli/venv/bin/activate
```

按当前系统/架构选择下载地址并安装：

```sh
source ~/.tccli/venv/bin/activate

case "$(uname -s)-$(uname -m)" in
  Darwin-x86_64) COSCLI_URL="https://cosbrowser.cloud.tencent.com/software/coscli/coscli-darwin-amd64" ;;
  Darwin-arm64) COSCLI_URL="https://cosbrowser.cloud.tencent.com/software/coscli/coscli-darwin-arm64" ;;
  Linux-i386|Linux-i686) COSCLI_URL="https://cosbrowser.cloud.tencent.com/software/coscli/coscli-linux-386" ;;
  Linux-x86_64) COSCLI_URL="https://cosbrowser.cloud.tencent.com/software/coscli/coscli-linux-amd64" ;;
  Linux-armv7l|Linux-armv6l) COSCLI_URL="https://cosbrowser.cloud.tencent.com/software/coscli/coscli-linux-arm" ;;
  Linux-aarch64|Linux-arm64) COSCLI_URL="https://cosbrowser.cloud.tencent.com/software/coscli/coscli-linux-arm64" ;;
  *) echo "unsupported platform: $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac

curl -L -o "$VIRTUAL_ENV/bin/coscli" "$COSCLI_URL"
chmod 755 "$VIRTUAL_ENV/bin/coscli"
coscli --version
```

## 安装 jq

`jq` 下载到虚拟环境的 `bin` 目录，并命名为 `jq`，这样激活虚拟环境后可以直接运行 `jq`。

先激活虚拟环境：

```sh
source ~/.tccli/venv/bin/activate
```

按当前系统/架构选择下载地址并安装：

```sh
source ~/.tccli/venv/bin/activate

case "$(uname -s)-$(uname -m)" in
  Darwin-x86_64) JQ_URL="https://github.com/jqlang/jq/releases/download/jq-1.8.1/jq-macos-amd64" ;;
  Darwin-arm64) JQ_URL="https://github.com/jqlang/jq/releases/download/jq-1.8.1/jq-macos-arm64" ;;
  Linux-x86_64) JQ_URL="https://github.com/jqlang/jq/releases/download/jq-1.8.1/jq-linux-amd64" ;;
  Linux-aarch64|Linux-arm64) JQ_URL="https://github.com/jqlang/jq/releases/download/jq-1.8.1/jq-linux-arm64" ;;
  *) echo "unsupported platform: $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac

curl -L -o "$VIRTUAL_ENV/bin/jq" "$JQ_URL"
chmod 755 "$VIRTUAL_ENV/bin/jq"
jq --version
```

## 验证环境

每次验证或执行命令前，先激活虚拟环境：

```bash
source ~/.tccli/venv/bin/activate
tccli --version 2>&1 | tr -d '\r'
coscli --version
jq --version
```

如果使用 COS 封装脚本，也先激活虚拟环境。脚本默认使用 `~/.tccli/venv/bin/tccli`、`~/.tccli/venv/bin/coscli` 和 `~/.tccli/venv/bin/python`：

```bash
source ~/.tccli/venv/bin/activate
scripts/coscli-auth.sh ls
```

如果在技能目录外调用脚本，使用实际脚本路径：

```bash
source ~/.tccli/venv/bin/activate
.agents/skills/tencent-cloud-governance/scripts/coscli-auth.sh ls
```

## 登录与凭证

`tccli` 使用临时密钥（OAuth/STS）登录，凭证由 tccli 管理并惰性自动刷新。先激活虚拟环境，再用下面的命令自检：

```sh
source ~/.tccli/venv/bin/activate
tccli sts GetCallerIdentity --region ap-shanghai 2>&1 | tr -d '\r'
```

如果出现 `secretId is invalid`、`credential:` 为空、缺少 `~/.tccli/<profile>.credential`，或脚本报 `tccli auth failed, re-login required`，先运行 OAuth 登录命令，触发授权页面让用户完成登录：

```sh
source ~/.tccli/venv/bin/activate
tccli auth login
```

登录命令可能会打开浏览器，或在终端输出授权 URL。用户完成授权后，再重新执行 `sts GetCallerIdentity` 自检。通过后，`coscli` 可通过 `scripts/coscli-auth.sh` 复用 tccli 凭证访问 COS。

如果临时 token 过期，先激活虚拟环境并重试一次 `tccli`，通常会刷新；连续鉴权失败再考虑重新登录。

## 日常命令模板

直接运行 `tccli`：

```sh
source ~/.tccli/venv/bin/activate
tccli cvm DescribeInstances --region ap-shanghai --Limit 100
```

通过封装脚本运行 `coscli`：

```sh
source ~/.tccli/venv/bin/activate
.agents/skills/tencent-cloud-governance/scripts/coscli-auth.sh ls cos://qibaotu-1313190940
```

如果写成单行命令，也要先 source 虚拟环境：

```sh
source ~/.tccli/venv/bin/activate && tccli sts GetCallerIdentity --region ap-shanghai
source ~/.tccli/venv/bin/activate && .agents/skills/tencent-cloud-governance/scripts/coscli-auth.sh ls
```
