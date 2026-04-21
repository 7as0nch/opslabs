#!/bin/bash
# hello-world 回归:确保故障/初始状态真的存在
# 在容器启动直后跑,此时 ready.flag 必须不存在
if [ -f /tmp/ready.flag ]; then
    echo "FAIL: flag 不应该一开始就存在"
    exit 1
fi
echo "PASS"
