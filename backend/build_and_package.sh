#!/bin/bash

# 构建Docker镜像
echo "正在构建Docker镜像..."
docker build -t aichat-backend:latest .

if [ $? -ne 0 ]; then
    echo "Docker镜像构建失败！"
    exit 1
fi

# 保存Docker镜像为tar文件
echo "正在保存Docker镜像..."
docker save -o aichat-backend.tar aichat-backend:latest

if [ $? -ne 0 ]; then
    echo "Docker镜像保存失败！"
    exit 1
fi

# 创建部署目录
echo "正在创建部署目录..."
mkdir -p deploy

# 复制必要文件到部署目录
echo "正在复制必要文件..."
cp aichat-backend.tar deploy/
cp docker-compose.yml deploy/

# 创建运行脚本
echo "正在创建运行脚本..."
cat > deploy/run.sh << 'EOF'
#!/bin/bash

# 检查是否已安装Docker
if ! command -v docker &> /dev/null; then
    echo "Docker未安装，正在安装..."
    # 安装Docker
    apt-get update
    apt-get install -y docker.io docker-compose
    # 启动Docker服务
    systemctl start docker
    systemctl enable docker
fi

# 检查是否已安装Docker Compose
if ! command -v docker-compose &> /dev/null; then
    echo "Docker Compose未安装，正在安装..."
    apt-get install -y docker-compose
fi

# 加载Docker镜像
echo "正在加载Docker镜像..."
docker load -i aichat-backend.tar

if [ $? -ne 0 ]; then
    echo "Docker镜像加载失败！"
    exit 1
fi

# 创建配置文件目录
echo "正在创建配置文件目录..."
mkdir -p /root/workspace/conf

# 创建日志目录
echo "正在创建日志目录..."
mkdir -p /root/workspace/logs

# 检查配置文件是否存在
if [ ! -f /root/workspace/conf/config.yaml ]; then
    echo "警告：配置文件 /root/workspace/conf/config.yaml 不存在，请确保已将配置文件放入该目录！"
    echo "您可以从项目的configs目录复制配置文件到该目录。"
fi

# 启动服务
echo "正在启动服务..."
docker-compose up -d

if [ $? -ne 0 ]; then
    echo "服务启动失败！"
    exit 1
fi

echo "服务启动成功！"
echo "HTTP端口：8000"
echo "gRPC端口：9000"
echo "配置文件目录：/root/workspace/conf"
echo "日志文件目录：/root/workspace/logs"
EOF

# 赋予运行脚本执行权限
chmod +x deploy/run.sh

# 压缩部署目录为tar.gz文件
echo "正在压缩部署目录..."
tar -czf aichat-backend-deploy.tar.gz deploy/

# 清理临时文件
echo "正在清理临时文件..."
rm -f aichat-backend.tar
rm -rf deploy/

echo "打包完成！"
echo "部署包：aichat-backend-deploy.tar.gz"
echo "使用方法："
echo "1. 将 aichat-backend-deploy.tar.gz 上传到目标服务器"
echo "2. 解压：tar -xzf aichat-backend-deploy.tar.gz"
echo "3. 进入目录：cd deploy"
echo "4. 运行：./run.sh"
