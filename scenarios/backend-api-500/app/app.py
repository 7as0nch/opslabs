#!/usr/bin/env python3
"""opslabs user-api —— Flask app,演示"配置错 → 看日志 → 修 → 重启"排障路径

故障设计:
  - 启动时读 /etc/app/config.yaml 的 db_password
  - verify_password() 模拟 PostgreSQL 客户端的密码验证(不真接 PG,SQLite 替身)
  - 密码不匹配时 GET /users/<id> 抛 OperationalError 让 Flask 走 500 + 日志写一行
    "FATAL: password authentication failed for user 'opslabs'"
  - 解法:cat /etc/app/config.yaml 看到密码,改成 opslabs2026,systemctl restart app

健康检查 /healthz 始终 200,方便用户先确认 listener 通了再排查业务路径。
"""
import logging
import os
import sqlite3
import sys
from pathlib import Path

import yaml
from flask import Flask, jsonify

# 我们模拟的"PostgreSQL 服务端密码" —— 用户改 config 时要把 db_password 改成这个值
EXPECTED_DB_PASSWORD = 'opslabs2026'

CONFIG_PATH = '/etc/app/config.yaml'
LOG_PATH = '/var/log/app/error.log'

logging.basicConfig(
    filename=LOG_PATH,
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
)
logger = logging.getLogger('app')


def load_config() -> dict:
    if not os.path.exists(CONFIG_PATH):
        logger.error('config file missing: %s', CONFIG_PATH)
        sys.exit(2)
    with open(CONFIG_PATH, encoding='utf-8') as f:
        cfg = yaml.safe_load(f) or {}
    # 设置 log level
    level_name = str(cfg.get('log_level', 'INFO')).upper()
    logger.setLevel(getattr(logging, level_name, logging.INFO))
    return cfg


def init_db(db_path: str) -> None:
    Path(db_path).parent.mkdir(parents=True, exist_ok=True)
    with sqlite3.connect(db_path) as conn:
        conn.execute('CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT, email TEXT)')
        conn.execute('INSERT OR IGNORE INTO users (id, name, email) VALUES (1, "alice", "alice@example.com")')
        conn.execute('INSERT OR IGNORE INTO users (id, name, email) VALUES (2, "bob",   "bob@example.com")')
        conn.commit()


class DbAuthError(Exception):
    """模拟 psycopg2.OperationalError("FATAL: password authentication failed")"""


def verify_password(provided: str) -> None:
    """模拟 PG 服务端密码校验。错就 raise,日志格式跟 psycopg2 实际报错对齐"""
    if provided != EXPECTED_DB_PASSWORD:
        raise DbAuthError(
            f"FATAL: password authentication failed for user 'opslabs' (got: '{provided}')"
        )


cfg = load_config()
init_db(cfg.get('db_path', '/var/lib/app/users.db'))
logger.info('app booting on port %s', cfg.get('listen_port', 8080))

app = Flask(__name__)


@app.get('/healthz')
def healthz():
    return jsonify(ok=True)


@app.get('/users/<int:user_id>')
def get_user(user_id: int):
    try:
        verify_password(str(cfg.get('db_password', '')))
    except DbAuthError as e:
        logger.error('%s', e)
        return jsonify(error='internal server error'), 500

    try:
        with sqlite3.connect(cfg.get('db_path', '/var/lib/app/users.db')) as conn:
            row = conn.execute(
                'SELECT id, name, email FROM users WHERE id = ?', (user_id,)
            ).fetchone()
    except Exception as e:  # noqa: BLE001
        logger.exception('db query failed: %s', e)
        return jsonify(error='internal server error'), 500

    if not row:
        return jsonify(error='not found'), 404
    return jsonify(id=row[0], name=row[1], email=row[2])


if __name__ == '__main__':
    port = int(cfg.get('listen_port', 8080))
    # use_reloader=False 让 PID 文件指向真实进程,不会被 werkzeug 双进程模式骗
    app.run(host='127.0.0.1', port=port, use_reloader=False, debug=False)
