#!/bin/bash
# hello-world 参考解法:用于 CI/开发自测
# 场景 CI 会先跑 regression(验预埋故障),再跑本脚本,最后跑 check.sh 必须返回 OK
touch /tmp/ready.flag
