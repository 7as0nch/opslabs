#!/bin/bash
# ops-nginx-upstream-fail entrypoint
set -e
/opt/opslabs/setup.sh
exec ttyd -W -p 7681 --writable bash -l
