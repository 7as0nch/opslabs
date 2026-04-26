# Nginx 反代 502 排查

## 背景

客服反馈网站打不开,你登上服务器检查。架构很简单:

```
Client → Nginx (:80) → app service (:8080)
```

当前情况:

- `curl http://localhost/` 返回 **502 Bad Gateway**
- Nginx 进程在跑(`systemctl status nginx` 显示 active)
- app 服务进程也在跑(`systemctl status app` 显示 active)

## 你的任务

找出问题并修复,让 `curl http://localhost/` 返回 200,响应体包含 `Hello from app`。

**约束**:

- 不要重装任何组件
- 不要改 `app/server.py` 源码
- 只调整配置和服务状态即可

## 提示

- 看 `/var/log/nginx/error.log` 里 upstream 报错就有线索
- 这题有多种解法,改 nginx 端口或改 app 端口都能让 check 通过
- 修改 nginx 配置后用 `systemctl reload nginx`(等价 `nginx -s reload`)就行,**不要 restart**

## 预期耗时

8-12 分钟。这是面试常见题型(Nginx 502 三大原因之一:upstream port mismatch)。
