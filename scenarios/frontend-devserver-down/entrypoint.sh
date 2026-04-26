#!/bin/bash
# frontend-devserver-down entrypoint:
#   以 player 身份:预埋故障(setup.sh) → 起 ttyd 终端给用户操作
#   判题时 DockerRunner 用 `docker exec -u root` 回 root 跑 check.sh,
#   check.sh 自己 curl localhost:3000,不依赖 player 的 shell 状态
set -e
/opt/opslabs/setup.sh
# 进入 webapp 目录,用户打开终端就在 /home/player/webapp,省一步 cd
cd /home/player/webapp
exec ttyd -W -p 7681 --writable bash -l
