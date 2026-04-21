#!/bin/bash
# hello-world check:要求 /tmp/ready.flag 存在
# 约定:
#   - stdout 首行 == "OK" 视为通关
#   - 其它输出视为未通过,写 stderr 方便给用户看
#   - 脚本本身退出码固定 0,失败语义靠 stdout 判断
set -o pipefail

if [ ! -f /tmp/ready.flag ]; then
    echo "NO"
    echo "flag 文件还未创建" >&2
    exit 0
fi

echo "OK"
exit 0
