package main

// 内嵌的精美 H5 脱敏前端面板（无任何密码及加密逻辑，100%原生纯本地渲染，绝不请求任何外网 CDN 加密库）
const htmlTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>万能大文件下载加速代理</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background-color: #f4f7f6; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; }
        .card { background: white; padding: 35px; border-radius: 16px; box-shadow: 0 10px 25px rgba(0,0,0,0.05); width: 100%; max-width: 500px; box-sizing: border-box; }
        h2 { margin-top: 0; color: #2c3e50; text-align: center; margin-bottom: 25px; }
        .input-group { margin: 20px 0; }
        label { display: block; margin-bottom: 8px; color: #7f8c8d; font-size: 14px; font-weight: 500; }
        input[type="text"], input[type="password"] { width: 100%; padding: 12px 14px; border: 1px solid #dcdfe6; border-radius: 8px; font-size: 14px; box-sizing: border-box; transition: all 0.3s; }
        input[type="text"]:focus, input[type="password"]:focus { border-color: #409eff; outline: none; box-shadow: 0 0 8px rgba(64,158,255,0.2); }
        button { width: 100%; background-color: #409eff; color: white; border: none; padding: 13px; font-size: 16px; border-radius: 8px; cursor: pointer; font-weight: bold; transition: background-color 0.3s; box-shadow: 0 4px 12px rgba(64,158,255,0.3); }
        button:hover { background-color: #66b1ff; }
        .warning-text { font-size: 11px; color: #409eff; background-color: #ecf5ff; padding: 10px; border-radius: 6px; margin-top: 15px; border: 1px solid #b3d8ff; line-height: 1.4; }
        .footer { font-size: 12px; color: #909399; text-align: center; margin-top: 25px; line-height: 1.5; }
    </style>
</head>
<body>
<div class="card">
    <h2>🚀 万能下载中转加速</h2>
    
    <!-- 使用原生的 POST 表单与后端握手，将密码隐藏在请求体(Body)中，彻底绝迹公网明文嗅探和日志泄漏 -->
    <form action="/api/get-ticket" method="POST" target="_blank" onsubmit="return validateForm()">
        <div class="input-group">
            <label checkfor="target">目标下载文件链接</label>
            <input type="text" id="target" name="target" placeholder="粘贴 http / https / 镜像 / 归档等任意大文件链接">
        </div>
        <div class="input-group">
            <label checkfor="password">访问通关密码</label>
            <input type="password" id="password" name="password" placeholder="请输入本站专属通关密码">
        </div>
        <button type="submit">立即全速下载</button>
    </form>

    <div class="warning-text">
        🔒 安全隔离提示：已启用内存安全随机凭证盾。全网公开盲扫、人肉拼接猜测路径将被系统无限制拦截。
    </div>
    <div class="footer">
        支持多线程工具 (IDM / 迅雷) 及各类手机浏览器断点续传<br>
        单文件支持上限：50GB
    </div>
</div>
<script>
    function validateForm() {
        let tgt = document.getElementById('target').value.trim();
        let pwd = document.getElementById('password').value.trim();
        if (!tgt) { alert('请先输入目标下载链接！'); return false; }
        if (!pwd) { alert('请先输入访问密码！'); return false; }
        return true;
    }
</script>
</body>
</html>`
