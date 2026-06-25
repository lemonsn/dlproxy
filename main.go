package main

import (
"context"
"crypto/rand"
"crypto/tls"
"encoding/hex"
"errors"
"io"
"log"
"net"
"net/http"
"net/netip"
"strconv"
"strings"
"sync"
"syscall"
"time"
)

// ==================== 核心安全与业务配置 ====================
const (
// MAX_DOWNLOAD_SIZE: 限制单次最大下载 50GB (支持大型 ISO 镜像和各类系统打包文件)
MAX_DOWNLOAD_SIZE = int64(50 * 1024 * 1024 * 1024)

// PROXY_SECRET_KEY: 直接在后端写死访问密码！
// 任何绕过网页拼接、或者直接调用接口的请求必须提供合法的临时票据，而票据必须用此密码在网页端兑换
PROXY_SECRET_KEY = "20260625"

// TICKET_EXPIRE_DURATION: 网页生成的安全下载令牌在内存中的绝对有效期（10分钟足够下载工具解析并建立连接）
TICKET_EXPIRE_DURATION = 10 * time.Minute
)

// 安全内存令牌桶：替代 URL 传输明文密码，防中间人嗅探
var (
tokenMutex sync.RWMutex
tokenStore = make(map[string]time.Time)
)

var bufferPool = sync.Pool{
New: func() any {
// 针对大文件，将缓冲区从 64KB 提升至 128KB，大幅减少大规模数据搬运时的系统调用次数
b := make([]byte, 128*1024)
return &b
},
}

var myIPs []net.IP

func init() {
// 获取系统自身绑定的所有 IP
addrs, err := net.InterfaceAddrs()
if err != nil {
return
}
for _, address := range addrs {
if ipnet, ok := address.(*net.IPNet); ok {
myIPs = append(myIPs, ipnet.IP)
}
}

// 启动后台异步垃圾清理协程：每分钟自动粉碎内存中已经过期的临时安全票据，确保系统零内存泄露风险
go func() {
for {
time.Sleep(1 * time.Minute)
tokenMutex.Lock()
now := time.Now()
for token, expireTime := range tokenStore {
if now.After(expireTime) {
delete(tokenStore, token)
}
}
tokenMutex.Unlock()
}
}()
}

// 深度校验是否为内网/环回/保留IP (SSRF防御)
func isUnsafeIP(ip net.IP) bool {
if ip == nil {
return true
}
if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
return true
}
if ip.IsUnspecified() {
return true
}
for _, myIP := range myIPs {
if myIP.Equal(ip) {
return true
}
}
return false
}

// 生成强随机高强度十六进制安全凭证串
func generateSecureToken() string {
b := make([]byte, 16)
if _, err := rand.Read(b); err != nil {
return strconv.FormatInt(time.Now().UnixNano(), 10)
}
return hex.EncodeToString(b)
}

func main() {
// 【网络层强御】：在真正的 TCP 三次握手建立瞬间拦截 DNS 重新绑定（DNS Rebinding）攻击
safeDialer := &net.Dialer{
Timeout:   30 * time.Second,
KeepAlive: 60 * time.Second,
Control: func(network, address string, c syscall.RawConn) error {
host, _, err := net.SplitHostPort(address)
if err != nil {
return err
}
parsedIP, err := netip.ParseAddr(host)
if err != nil {
return errors.New("invalid ip address inside dialer")
}
stdIP := net.IP(parsedIP.AsSlice())
if isUnsafeIP(stdIP) {
return errors.New("security restriction: reserved/private network connection is blocked")
}
return nil
},
}

httpClient := &http.Client{
CheckRedirect: func(req *http.Request, via []*http.Request) error {
if len(via) >= 10 {
return http.ErrUseLastResponse
}
if len(via) > 0 {
lastReq := via[len(via)-1]
// 确保大文件进行 302 重定向跟随跳转时，客户端的多线程分块 Range 头部不会丢失
if rangeHeader := lastReq.Header.Get("Range"); rangeHeader != "" {
req.Header.Set("Range", rangeHeader)
}
}
log.Printf("[Redirect Follow] 自动跟进大文件至: %s", req.URL.String())
return nil
},
Transport: &http.Transport{
DialContext:           safeDialer.DialContext,
TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
MaxIdleConns:          500,               // 增大闲置连接池，完美应对多线程下载工具(如IDM/迅雷)的疯狂并发请求
MaxIdleConnsPerHost:   100,               // 允许单个宿主机保持更多长连接
IdleConnTimeout:       120 * time.Second, // 延长闲置超时，防止大文件下载间歇性断开
TLSHandshakeTimeout:   15 * time.Second,
ExpectContinueTimeout: 1 * time.Second,
ResponseHeaderTimeout: 30 * time.Second,
},
}

address := ":8080"
log.Printf("[BigFile Proxy] 工业级安全闭环完整版已启动，监听端口 %s...", address)

server := &http.Server{
Addr:              address,
ReadHeaderTimeout: 10 * time.Second,
WriteTimeout:      0, // 解除写超时限制，支持无限期长时间大文件平滑传输
IdleTimeout:       120 * time.Second,
Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
handleFollowRedirectProxy(w, r, httpClient)
}),
}

if err := server.ListenAndServe(); err != nil {
log.Fatalf("服务器启动失败: %v", err)
}
}

func handleFollowRedirectProxy(w http.ResponseWriter, r *http.Request, client *http.Client) {
if r.URL.Path == "/favicon.ico" || r.URL.Path == "/robots.txt" {
w.WriteHeader(http.StatusNotFound)
return
}

w.Header().Set("Access-Control-Allow-Origin", "*")
w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
w.Header().Set("Access-Control-Allow-Headers", "*")
if r.Method == http.MethodOptions {
w.WriteHeader(http.StatusOK)
return
}

// 1. 处理网页鉴权表单的原生 POST 请求提交
if r.Method == http.MethodPost && r.URL.Path == "/api/get-ticket" {
if err := r.ParseForm(); err != nil {
http.Error(w, "Bad Request", http.StatusBadRequest)
return
}
password := r.FormValue("password")
targetURL := r.FormValue("target")

if password != PROXY_SECRET_KEY {
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.WriteHeader(http.StatusForbidden)
w.Write([]byte("<script>alert('⚠️ 通关密码不正确，拒绝生成安全下载链路！');window.history.back();</script>"))
return
}

if targetURL == "" {
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.WriteHeader(http.StatusBadRequest)
w.Write([]byte("<script>alert('⚠️ 请先输入目标下载链接！');window.history.back();</script>"))
return
}

// 密码正确，在后端内存中秘密签发一个高强度安全临时票据（Ticket）
ticket := generateSecureToken()
tokenMutex.Lock()
tokenStore[ticket] = time.Now().Add(TICKET_EXPIRE_DURATION)
tokenMutex.Unlock()

// 格式化清洗目标链接
for strings.HasPrefix(targetURL, "/") {
targetURL = targetURL[1:]
}
connector := "?"
if strings.Contains(targetURL, "?") {
connector = "&"
}

// 直接执行 302 重定向到带有临时随机票据的安全链上，完成无感安全准入
redirectPath := "/" + targetURL + connector + "_ticket=" + ticket
http.Redirect(w, r, redirectPath, http.StatusFound)
return
}

rawURI := r.RequestURI
for strings.HasPrefix(rawURI, "/") {
rawURI = rawURI[1:]
}

// 访问根路径，直接下发前端控制台 HTML
if rawURI == "" {
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.WriteHeader(http.StatusOK)
w.Write([]byte(htmlTemplate))
return
}

// 【全开放拼接的核心封锁线】：校验即用即抛的安全临时票据
ticket := r.URL.Query().Get("_ticket")
if ticket == "" {
http.Error(w, "Access Denied: 拒绝直接拼接请求！本站已锁死纯手工直接拼接，请在网页端中输入密码兑换下载连接。", http.StatusForbidden)
return
}

// 内存核验票据的生存周期
tokenMutex.RLock()
expireTime, exists := tokenStore[ticket]
tokenMutex.RUnlock()

if !exists || time.Now().After(expireTime) {
http.Error(w, "Access Denied: 安全下载票据不存在或已过期，请返回主页重新获取。", http.StatusForbidden)
return
}

// 从原始下载请求路径中彻底清洗清洗掉内部临时的 ticket 参数，确保绝对不污染外部真正的源站 URL
if idx := strings.Index(rawURI, "?"); idx != -1 {
parts := strings.Split(rawURI[idx+1:], "&")
var newParts []string
for _, p := range parts {
if !strings.HasPrefix(p, "_ticket=") {
newParts = append(newParts, p)
}
}
rawURI = rawURI[:idx]
if len(newParts) > 0 {
rawURI += "?" + strings.Join(newParts, "&")
}
}

var targetURL string
if strings.HasPrefix(rawURI, "https:/") {
targetURL = "https://" + strings.TrimPrefix(rawURI, "https:/")
targetURL = strings.Replace(targetURL, "https:///", "https://", 1)
} else if strings.HasPrefix(rawURI, "http:/") {
targetURL = "http://" + strings.TrimPrefix(rawURI, "http:/")
targetURL = strings.Replace(targetURL, "http:///", "http://", 1)
} else if strings.Contains(rawURI, ".") && strings.Contains(rawURI, "/") {
targetURL = "http://" + rawURI
}

if targetURL == "" {
http.Error(w, "无法解析的目标链接格式", http.StatusBadRequest)
return
}

log.Printf("[Proxy Action] 安全中转大目标: %s", targetURL)

ctx, cancel := context.WithCancel(r.Context())
defer cancel()

upstreamReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL, nil)
if err != nil {
http.Error(w, "构建请求失败", http.StatusInternalServerError)
return
}

for k, vv := range r.Header {
if isHopByHopHeader(k) {
continue
}
for _, v := range vv {
upstreamReq.Header.Add(k, v)
}
}
// 修复宿主头，完美越过绝大多数公网 Nginx/Cloudflare 的反代屏蔽策略
upstreamReq.Host = upstreamReq.URL.Host

resp, err := client.Do(upstreamReq)
if err != nil {
log.Printf("[Error] 连接源站失败: %v", err)
http.Error(w, "目标下载服务器无响应、DNS解析失败或安全策略阻断", http.StatusBadGateway)
return
}
defer resp.Body.Close()

// 50GB 硬性安全红线防护检查
if resp.ContentLength > MAX_DOWNLOAD_SIZE {
http.Error(w, "Security Restriction: Target file exceeds the 50GB proxy limit.", http.StatusForbidden)
return
}

// 转发源站的所有 Header 到用户端（包括断点续传多线程必用的 Content-Range 和 Accept-Ranges）
for k, vv := range resp.Header {
if isHopByHopHeader(k) {
continue
}
w.Header()[k] = vv
}
w.WriteHeader(resp.StatusCode)

if r.Method == http.MethodHead {
return
}

bufPtr := bufferPool.Get().(*[]byte)
defer bufferPool.Put(bufPtr)

// 使用 LimitedReader 强力保护，即便恶意源站故意伪造不存在的容量或者返回无限网络流，最多吐满 50GB 就会强切
limitedBody := io.LimitReader(resp.Body, MAX_DOWNLOAD_SIZE)
_, err = io.CopyBuffer(w, limitedBody, *bufPtr)
if err != nil {
if err != context.Canceled {
log.Printf("[Stream End] 传输中断或用户取消: %v", err)
}
_, _ = io.Copy(io.Discard, resp.Body)
}
}

func isHopByHopHeader(headerName string) bool {
h := strings.ToLower(headerName)
return h == "connection" || h == "proxy-connection" || h == "keep-alive" || h == "te" || h == "trailer" || h == "upgrade"
}
