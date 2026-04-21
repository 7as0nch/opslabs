# opslabs 开发工作流(POSIX 系统:Linux / macOS / WSL / Git Bash)
# Windows 原生 cmd/PowerShell 请使用 make.ps1 代替,target 名对应

.PHONY: help dev dev-backend dev-frontend gen scenarios scenarios-test smoke install-frontend

help:
	@echo 'opslabs Makefile'
	@echo ''
	@echo 'targets:'
	@echo '  make dev-backend      # 启动后端 (默认 mock runtime)'
	@echo '  make dev-frontend     # 启动前端 vite (http://localhost:5173)'
	@echo '  make dev              # 前后端并行启动'
	@echo '  make install-frontend # 首次 npm install'
	@echo '  make gen              # 重新生成 proto / wire'
	@echo '  make scenarios        # docker build 全部场景镜像'
	@echo '  make scenarios-test   # 对 hello-world 跑 regression+solution+check'
	@echo '  make smoke            # curl 打一遍后端 REST API'

# ---------- 开发启动 ----------

dev-backend:
	cd backend && go run ./cmd/backend -conf configs/config.yaml

dev-frontend:
	cd frontend && npm run dev

# 并行启动前后端;ctrl-C 一起退出
dev:
	@command -v make >/dev/null 2>&1 || { echo "make not found"; exit 1; }
	$(MAKE) -j2 dev-backend dev-frontend

install-frontend:
	cd frontend && npm install

# ---------- 代码生成 ----------

# proto + wire 都是 Windows PowerShell 脚本,POSIX 下需要 pwsh;否则用各自的 bash 版
gen:
	@if command -v pwsh >/dev/null 2>&1; then \
		pwsh -NoProfile -File backend/scripts/gen-api.ps1; \
		pwsh -NoProfile -File backend/scripts/wire.ps1; \
	else \
		echo 'pwsh not installed; run gen-api.ps1 + wire.ps1 on Windows manually'; \
	fi

# ---------- 场景镜像 ----------

scenarios:
	bash scripts/build-all-scenarios.sh

scenarios-test:
	bash scripts/test-scenario.sh hello-world

# ---------- 冒烟测试(Phase A smoke) ----------

smoke:
	bash scripts/curl-smoke.sh
