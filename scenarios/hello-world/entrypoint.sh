#!/bin/bash
# hello-world entrypoint:
#   容器默认用户已在 Dockerfile 里设成 player(USER player),
#   这里直接以 player 身份跑 setup + ttyd,避免引入 su-exec / su。
#   好处:
#     - 不依赖 CAP_SETUID/CAP_SETGID(--cap-drop=ALL 下这两个也没了)
#     - /home/player 的 tmpfs 本来就是 player 所有,写文件天然不踩 DAC
#   判题时 DockerRunner 会用 `docker exec -u root` 回到 root 跑 check.sh,
#   这条路径由 dockerd 管,不受容器内 caps 限制。
set -e
/opt/opslabs/setup.sh
exec ttyd -W -p 7681 --writable bash -l
