#!/bin/bash
# backend-api-500 entrypoint:setup → ttyd
#   setup.sh 已经把 app 起来(故障状态);check.sh 走 docker exec -u root,
#   直接 curl localhost:8080 不依赖 player shell 状态。
set -e
/opt/opslabs/setup.sh
exec ttyd -W -p 7681 --writable bash -l
