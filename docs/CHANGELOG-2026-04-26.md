# 变更记录 · 2026-04-26

> 紧接 [CHANGELOG-2026-04-25.md](CHANGELOG-2026-04-25.md) —— 用户实测触发的 bug 修复 + 国内构建网络适配。

---

## 一、Bug 修复

### 1.1 [Bug] WebContainer 构建失败:Ubuntu apt 源国内拉不动

**现象**:执行 `./scripts/build-all-scenarios.ps1` 时,scenarios/backend-api-500 构建在 `apt-get update` 阶段挂死:

```
E: Failed to fetch http://security.ubuntu.com/ubuntu/dists/jammy-security/.../Packages
   Connection failed [IP: 185.125.190.83 80]
E: Failed to fetch http://archive.ubuntu.com/ubuntu/dists/jammy/.../Packages
   Connection failed [IP: 91.189.91.81 80]
exit code: 100
```

**根因**:Ubuntu 官方 apt 源(archive.ubuntu.com / security.ubuntu.com)在国内访问极不稳定,Docker build 默认走的就是这两个源,网络抖动一次整个构建链就挂。base 镜像 + 三个新场景层都受影响。

**修法**:在 base 镜像层一次性把 apt 源切到阿里云镜像,所有 FROM `opslabs/base:v1` 的下游场景自动受益。同时给 base-minimal(Alpine)和 frontend-devserver-down 的 npm 都做了对应处理:

| 文件 | 改动 |
|---|---|
| [scenarios-build/base/Dockerfile](../scenarios-build/base/Dockerfile) | apt sources.list 切到 `mirrors.aliyun.com/ubuntu/`(支持 `--build-arg APT_MIRROR=archive.ubuntu.com` 海外切回官方) |
| [scenarios-build/base-minimal/Dockerfile](../scenarios-build/base-minimal/Dockerfile) | apk repositories 切到 `mirrors.aliyun.com/alpine`(支持 `--build-arg APK_MIRROR=...` 切回) |
| [scenarios/frontend-devserver-down/Dockerfile](../scenarios/frontend-devserver-down/Dockerfile) | 写 `/etc/npmrc` 把 npm registry 切到 `registry.npmmirror.com`,docker build 期 + 用户解题期都生效;支持 `--build-arg NPM_REGISTRY=...` 切回 |

**用户操作(重要)**:

base 镜像已经在本地存在但**没带新源**,直接重跑 build 会命中缓存 → 仍走老 base。需要先**删旧 base 镜像**强制重 build:

```powershell
# Windows PowerShell
docker rmi -f opslabs/base:v1 opslabs/base-minimal:v1
./scripts/build-all-scenarios.ps1
```

```bash
# POSIX
docker rmi -f opslabs/base:v1 opslabs/base-minimal:v1
./scripts/build-all-scenarios.sh
```

构建顺序会被脚本扫描自动安排好:scenarios-build/base 和 base-minimal 先 build → scenarios/* 后 build。

### 1.2 海外 / CI 环境切回官方源

如果在海外或 CI 上构建反而被阿里云镜像拖慢(比如 GitHub Actions 跑 us-east),用 `--build-arg` 临时切回:

```bash
docker build \
  --build-arg APT_MIRROR=archive.ubuntu.com \
  -t opslabs/base:v1 scenarios-build/base
```

`build-all-scenarios.{sh,ps1}` 暂时不传 build-arg(99% 用户在国内,默认体验最优);需要切换的开发者直接手跑 `docker build` 即可。

---

## 二、本次涉及文件

```
scenarios-build/base/Dockerfile               (新增 sed 切 apt 源)
scenarios-build/base-minimal/Dockerfile        (新增 sed 切 apk 源)
scenarios/frontend-devserver-down/Dockerfile  (新增 /etc/npmrc 切 npm registry)
```

---

## 三、还没验证的

我这边没真跑 `docker build`(沙箱无 Docker daemon),修改是按照 Dockerfile 语法 + 阿里云镜像清单做的。**请按 § 1.1 的步骤本地重跑一次 `build-all-scenarios.ps1`**,把所有镜像跑通,再跑各场景的 `tests/regression.sh` 闭环验证。

如果某条还卡住,大概率是 NodeSource(`deb.nodesource.com`)某次抖动 —— 这个走 fastly CDN 国内一般通,真不通的话再换"直接下载 Node prebuilt 二进制"方案。
