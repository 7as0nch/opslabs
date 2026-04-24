# wasm-linux-hello / vendor

这个目录存 v86 运行必需的二进制资源。**默认不纳入版本控制**(.gitignore
里应忽略,见 repo 根目录),因为:

- 体积合计 ~5MB,不适合常驻 git 历史
- 来源是 copy.sh/v86 的官方 release(GPLv2),合规上更建议用脚本拉取

## 跑一次 fetch 脚本就好

```bash
# Linux / macOS
./scripts/fetch-v86.sh

# Windows PowerShell
./scripts/fetch-v86.ps1
```

## 期望产物

脚本保留 copy.sh 的目录结构(`build/` `bios/` `images/`),让前端 iframe
加载时 `./vendor/build/v86.wasm` 与 `https://copy.sh/v86/build/v86.wasm`
URL 后缀对齐,`bootLib` 不需要按源做路径翻译。

```
vendor/
├── build/
│   ├── libv86.js     ~800 KB  v86 JS 主库
│   └── v86.wasm      ~500 KB  WebAssembly JIT 核心
├── bios/
│   ├── seabios.bin   ~96  KB  BIOS(x86 开机)
│   └── vgabios.bin   ~40  KB  VGA BIOS(显卡 ROM)
└── images/
    └── linux.iso     ~4   MB  BusyBox + musl 精简镜像(cdrom 启动)
```

## 为什么不直接用 CDN

`copy.sh` 在部分网络(比如国内默认线路)会被运营商拦截或干扰,
表现为页面 `libv86.js 未加载成功` 文案。把资源挪到后端 embed.FS
下发是最稳的办法:

- Go 二进制里带着这 5MB,部署哪儿都能跑
- 前端 iframe 通过 `./vendor/xxx` 相对路径走同源请求,不跨域
- 给后端单独换镜像(比如公司内网) ? 只改 `scripts/fetch-v86.sh`
  的 `V86_MIRROR`,不碰业务代码
