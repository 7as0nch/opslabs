#!/bin/bash
# hello-world setup:预埋 welcome 文件 + 欢迎语
# 注意:本脚本由 entrypoint.sh 以 player 身份调用
#      因此:
#        - 不做 chown(我们就是 player,文件默认就是 player 所有)
#        - 不做 chmod 0644(umask 022 的默认落点就是 0644)
#        - 任何需要 CAP_CHOWN/CAP_DAC_OVERRIDE 的操作都不该出现在这里
set -e

cat > /home/player/welcome.txt <<'EOF'
欢迎来到 opslabs。

你的第一个任务:
在 /tmp 下创建一个名为 ready.flag 的空文件,
然后点击界面上的「检查答案」按钮。

提示:
  - 创建空文件可以用 touch 命令
  - 例如: touch /tmp/example.txt
EOF

cat >> /home/player/.bashrc <<'EOF'

echo ""
echo "========================================"
echo "  欢迎!请查看 ~/welcome.txt"
echo "========================================"
echo ""
EOF

exit 0
