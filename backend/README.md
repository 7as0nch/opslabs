# Kratos Project Template

## Install Kratos
```
go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
```
## Create a service
```
# Create a template project
kratos new server

cd server
# Add a proto template
kratos proto add api/server/server.proto
# Generate the proto code
kratos proto client api/server/server.proto
# Generate the source code of service by proto file
kratos proto server api/server/server.proto -t internal/service

go generate ./...
go build -o ./bin/ ./...
./bin/server -conf ./configs
```
## Generate other auxiliary files by Makefile
```
# Download and update dependencies
make init
# Generate API files (include: pb.go, http, grpc, validate, swagger) by proto file
make api
# Generate all files
make all
```
## Automated Initialization (wire)
```
# install wire
go get github.com/google/wire/cmd/wire

# generate wire
cd cmd/server
wire
```

## Docker
```bash
# build
docker build -t <your-docker-image-name> .

# run
docker run --rm -p 8000:8000 -p 9000:9000 -v </path/to/your/configs>:/data/conf <your-docker-image-name>
```

## 部署流程

### 1. 构建部署包
```bash
# 在项目根目录执行构建脚本
./build_and_package.sh
```

### 2. 上传部署包到目标服务器
```bash
# 使用scp命令上传部署包到目标服务器
scp aichat-backend-deploy.tar.gz root@your-server-ip:/root/
```

### 3. 在目标服务器上解压和运行
```bash
# 登录目标服务器
ssh root@your-server-ip

# 解压部署包
tar -xzf aichat-backend-deploy.tar.gz

# 进入部署目录
cd deploy

# 运行部署脚本
./run.sh
```

### 4. 配置和日志管理

#### 配置文件
- 配置文件目录：`/root/workspace/conf`
- 主要配置文件：`/root/workspace/conf/config.yaml`
- 可以通过修改该文件来调整服务配置

#### 日志文件
- 日志文件目录：`/root/workspace/logs`
- 服务日志会自动写入该目录
- 可以通过配置文件调整日志级别和格式

### 5. 服务管理

#### 查看服务状态
```bash
docker-compose ps
```

#### 查看服务日志
```bash
docker-compose logs -f
```

#### 停止服务
```bash
docker-compose down
```

#### 重启服务
```bash
docker-compose restart
```

### 6. 访问服务
- HTTP端口：`8000`
- gRPC端口：`9000`
- 可以通过 `http://your-server-ip:8000` 访问HTTP接口
- 可以通过 `your-server-ip:9000` 访问gRPC接口


