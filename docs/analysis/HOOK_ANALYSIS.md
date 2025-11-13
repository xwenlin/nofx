# Hook 系统架构与使用流程分析

## 目录

1. [系统架构概览](#1-系统架构概览)
2. [核心组件](#2-核心组件)
3. [注册流程](#3-注册流程)
4. [执行流程](#4-执行流程)
5. [现有 Hook 扩展点详解](#5-现有-hook-扩展点详解)
6. [完整使用流程](#6-完整使用流程)
7. [设计特点](#7-设计特点)
8. [最佳实践](#8-最佳实践)
9. [添加新 Hook 扩展点](#9-添加新-hook-扩展点)
10. [总结](#10-总结)

---

## 1. 系统架构概览

Hook 系统提供了一个通用的扩展点机制，允许在不修改核心代码的前提下注入自定义逻辑。

### 架构图

```
┌─────────────────────────────────────────────────────────┐
│                    Hook 系统架构                          │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  ┌──────────────┐         ┌──────────────┐              │
│  │  注册阶段     │  ────>  │  执行阶段     │              │
│  │ RegisterHook │         │  HookExec    │              │
│  └──────────────┘         └──────────────┘              │
│         │                        │                       │
│         │                        │                       │
│         v                        v                       │
│  ┌──────────────────────────────────────────┐           │
│  │        全局 Hook 注册表 (Hooks map)        │           │
│  │  - GETIP                                  │           │
│  │  - NEW_BINANCE_TRADER                     │           │
│  │  - NEW_ASTER_TRADER                       │           │
│  │  - SET_HTTP_CLIENT                        │           │
│  └──────────────────────────────────────────┘           │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

### 核心特点

- ✅ **类型安全的泛型 API** - 使用 Go 泛型确保类型安全
- ✅ **自动 Fallback** - Hook 未注册时自动回退到默认逻辑
- ✅ **支持任意参数和返回值** - 灵活的参数传递
- ✅ **全局开关控制** - 可通过 `EnableHooks` 禁用所有 Hook
- ✅ **详细日志记录** - 自动记录 Hook 执行状态

---

## 2. 核心组件

### 2.1 Hook 注册表

**位置**: `hook/hooks.go`

```go
var (
    Hooks       map[string]HookFunc = map[string]HookFunc{}
    EnableHooks                     = true
)
```

- **`Hooks`**: 全局注册表，存储所有已注册的 Hook 函数
  - Key: Hook 常量（如 `GETIP`、`NEW_BINANCE_TRADER`）
  - Value: Hook 函数实现
- **`EnableHooks`**: 全局开关，可禁用所有 Hook（用于调试或紧急情况）

### 2.2 Hook 函数类型

```go
type HookFunc func(args ...any) any
```

- 支持任意数量的参数（`args ...any`）
- 返回任意类型（`any`）
- 使用泛型在调用时保证类型安全

### 2.3 Result 类型接口

所有 Hook 返回的 Result 类型都应实现以下方法：

```go
type Result interface {
    Error() error        // 返回错误（如果有）
    GetResult() T        // 返回实际结果
}
```

示例实现：

```go
type IpResult struct {
    Err error
    IP  string
}

func (r *IpResult) Error() error {
    return r.Err
}

func (r *IpResult) GetResult() string {
    if r.Err != nil {
        log.Printf("⚠️ 执行GetIP时出错: %v", r.Err)
    }
    return r.IP
}
```

---

## 3. 注册流程

### 3.1 注册 API

**函数签名**:

```go
func RegisterHook(key string, hook HookFunc)
```

**参数**:
- `key`: Hook 常量（如 `hook.GETIP`）
- `hook`: Hook 函数实现

**实现**:

```go
func RegisterHook(key string, hook HookFunc) {
    Hooks[key] = hook
}
```

### 3.2 注册示例

#### 示例 1: 注册 IP 获取 Hook

```go
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    userId := args[0].(string)
    
    // 自定义逻辑：获取用户专用 IP
    proxyIP, err := getProxyIP(userId)
    
    return &hook.IpResult{
        Err: err,
        IP:  proxyIP,
    }
})
```

#### 示例 2: 注册 Binance 客户端 Hook

```go
hook.RegisterHook(hook.NEW_BINANCE_TRADER, func(args ...any) any {
    userId := args[0].(string)
    client := args[1].(*futures.Client)
    
    // 修改客户端配置（添加代理）
    if client.HTTPClient != nil {
        client.HTTPClient.Transport = getProxyTransport(userId)
    }
    
    return &hook.NewBinanceTraderResult{
        Client: client,
    }
})
```

### 3.3 注册时机

Hook 可以在以下时机注册：

1. **模块初始化时**（`init()` 函数）
   ```go
   func init() {
       hook.RegisterHook(hook.GETIP, myHookFunc)
   }
   ```

2. **应用启动时**（根据配置条件）
   ```go
   func InitHooks(enabled bool) {
       if !enabled {
           return
       }
       hook.RegisterHook(hook.GETIP, myHookFunc)
   }
   ```

3. **运行时动态注册**（不推荐，除非有特殊需求）

**推荐做法**: 在模块的 `InitHooks()` 函数中注册，便于管理和控制。

---

## 4. 执行流程

### 4.1 执行 API

**函数签名**:

```go
func HookExec[T any](key string, args ...any) *T
```

**参数**:
- `key`: Hook 常量
- `args`: 传递给 Hook 函数的参数

**返回值**:
- `*T`: 泛型类型指针，如果 Hook 未注册或执行失败，返回 `nil`

### 4.2 执行逻辑

```go
func HookExec[T any](key string, args ...any) *T {
    // 1. 检查全局开关
    if !EnableHooks {
        log.Printf("🔌 Hooks are disabled, skip hook: %s", key)
        var zero *T
        return zero
    }
    
    // 2. 查找 Hook
    if hook, exists := Hooks[key]; exists && hook != nil {
        log.Printf("🔌 Execute hook: %s", key)
        
        // 3. 执行 Hook 函数
        res := hook(args...)
        
        // 4. 类型断言并返回
        return res.(*T)
    } else {
        log.Printf("🔌 Do not find hook: %s", key)
    }
    
    // 5. Hook 未注册，返回 nil
    var zero *T
    return zero
}
```

### 4.3 执行示例

#### 示例 1: 获取用户 IP

```go
// api/server.go
func (s *Server) handleGetServerIP(c *gin.Context) {
    // 调用 Hook
    userIP := hook.HookExec[hook.IpResult](
        hook.GETIP, 
        c.GetString("user_id"),
    )
    
    // 检查结果
    if userIP != nil && userIP.Error() == nil {
        // 使用 Hook 返回的 IP
        c.JSON(http.StatusOK, gin.H{
            "public_ip": userIP.GetResult(),
            "message":   "请将此IP地址添加到白名单中",
        })
        return
    }
    
    // Fallback：使用默认逻辑
    publicIP := getPublicIPFromAPI()
    // ...
}
```

#### 示例 2: 创建 Binance 客户端

```go
// trader/binance_futures.go
func NewFuturesTrader(apiKey, secretKey string, userId string) *FuturesTrader {
    client := futures.NewClient(apiKey, secretKey)
    
    // 调用 Hook，允许修改客户端配置
    hookRes := hook.HookExec[hook.NewBinanceTraderResult](
        hook.NEW_BINANCE_TRADER, 
        userId, 
        client,
    )
    
    if hookRes != nil && hookRes.GetResult() != nil {
        client = hookRes.GetResult()
    }
    
    // 继续使用 client...
}
```

### 4.4 执行流程图

```
┌─────────────────┐
│  业务代码调用    │
│  HookExec()     │
└────────┬────────┘
         │
         v
    ┌─────────┐
    │检查开关  │ EnableHooks == true?
    └────┬────┘
         │ Yes
         v
    ┌─────────┐
    │查找Hook │ Hook 存在?
    └────┬────┘
         │ Yes
         v
    ┌─────────┐
    │执行Hook │
    └────┬────┘
         │
         v
    ┌─────────┐
    │返回结果  │
    └─────────┘
         │
         v
    ┌─────────┐
    │业务代码  │ 检查 result != nil && result.Error() == nil
    │处理结果  │
    └─────────┘
```

---

## 5. 现有 Hook 扩展点详解

### 5.1 GETIP - 获取用户 IP

**用途**: 返回用户专用 IP（如代理 IP）

**调用位置**: `api/server.go:212`

**Hook 常量**: `hook.GETIP`

**函数签名**: `func (userID string) *IpResult`

**参数**:
- `userID string` - 用户 ID

**返回类型**: `*hook.IpResult`

```go
type IpResult struct {
    Err error
    IP  string
}
```

**使用场景**:
- 为不同用户返回不同的代理 IP
- 动态 IP 分配
- IP 白名单配置

**实际调用代码**:

```go
// api/server.go
userIP := hook.HookExec[hook.IpResult](hook.GETIP, c.GetString("user_id"))
if userIP != nil && userIP.Error() == nil {
    c.JSON(http.StatusOK, gin.H{
        "public_ip": userIP.GetResult(),
        "message":   "请将此IP地址添加到白名单中",
    })
    return
}
```

**Fallback 逻辑**: 如果 Hook 未注册或执行失败，系统会：
1. 尝试通过第三方 API 获取公网 IP
2. 从网络接口获取第一个公网 IP
3. 如果都失败，返回错误

---

### 5.2 NEW_BINANCE_TRADER - Binance 客户端创建

**用途**: 在创建 Binance 交易客户端时注入自定义配置

**调用位置**: `trader/binance_futures.go:68`

**Hook 常量**: `hook.NEW_BINANCE_TRADER`

**函数签名**: `func (userID string, client *futures.Client) *NewBinanceTraderResult`

**参数**:
- `userID string` - 用户 ID
- `client *futures.Client` - Binance 客户端实例

**返回类型**: `*hook.NewBinanceTraderResult`

```go
type NewBinanceTraderResult struct {
    Err    error
    Client *futures.Client
}
```

**使用场景**:
- 为客户端设置代理
- 添加自定义日志记录器
- 修改 HTTP 传输配置
- 添加请求/响应拦截器

**实际调用代码**:

```go
// trader/binance_futures.go
func NewFuturesTrader(apiKey, secretKey string, userId string) *FuturesTrader {
    client := futures.NewClient(apiKey, secretKey)
    
    hookRes := hook.HookExec[hook.NewBinanceTraderResult](
        hook.NEW_BINANCE_TRADER, 
        userId, 
        client,
    )
    
    if hookRes != nil && hookRes.GetResult() != nil {
        client = hookRes.GetResult()
    }
    
    // 继续使用 client...
}
```

**注册示例**:

```go
hook.RegisterHook(hook.NEW_BINANCE_TRADER, func(args ...any) any {
    userId := args[0].(string)
    client := args[1].(*futures.Client)
    
    // 设置代理
    if client.HTTPClient != nil {
        client.HTTPClient.Transport = &http.Transport{
            Proxy: http.ProxyURL(getProxyURL(userId)),
        }
    }
    
    return &hook.NewBinanceTraderResult{
        Client: client,
    }
})
```

---

### 5.3 NEW_ASTER_TRADER - Aster 客户端创建

**用途**: 在创建 Aster 交易客户端时注入自定义配置

**调用位置**: `trader/aster_trader.go:68`

**Hook 常量**: `hook.NEW_ASTER_TRADER`

**函数签名**: `func (user string, client *http.Client) *NewAsterTraderResult`

**参数**:
- `user string` - 用户名
- `client *http.Client` - HTTP 客户端实例

**返回类型**: `*hook.NewAsterTraderResult`

```go
type NewAsterTraderResult struct {
    Err    error
    Client *http.Client
}
```

**使用场景**:
- 为 Aster 客户端设置代理
- 配置超时时间
- 添加自定义传输层

**实际调用代码**:

```go
// trader/aster_trader.go
func NewAsterTrader(user, signer, privateKey string) (*AsterTrader, error) {
    client := &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            TLSHandshakeTimeout:   10 * time.Second,
            ResponseHeaderTimeout: 10 * time.Second,
            IdleConnTimeout:       90 * time.Second,
        },
    }
    
    res := hook.HookExec[hook.NewAsterTraderResult](
        hook.NEW_ASTER_TRADER, 
        user, 
        client,
    )
    
    if res != nil && res.Error() == nil {
        client = res.GetResult()
    }
    
    // 继续使用 client...
}
```

---

### 5.4 SET_HTTP_CLIENT - HTTP 客户端设置

**用途**: 为市场数据 API 客户端设置自定义 HTTP 客户端

**调用位置**: `market/api_client.go:27`

**Hook 常量**: `hook.SET_HTTP_CLIENT`

**函数签名**: `func (client *http.Client) *SetHttpClientResult`

**参数**:
- `client *http.Client` - HTTP 客户端实例

**返回类型**: `*hook.SetHttpClientResult`

```go
type SetHttpClientResult struct {
    Err    error
    Client *http.Client
}
```

**使用场景**:
- 为市场数据 API 设置代理
- 配置请求超时
- 添加自定义传输层

**实际调用代码**:

```go
// market/api_client.go
func NewAPIClient() *APIClient {
    client := &http.Client{
        Timeout: 30 * time.Second,
    }
    
    hookRes := hook.HookExec[hook.SetHttpClientResult](
        hook.SET_HTTP_CLIENT, 
        client,
    )
    
    if hookRes != nil && hookRes.Error() == nil {
        log.Printf("使用Hook设置的HTTP客户端")
        client = hookRes.GetResult()
    }
    
    return &APIClient{
        client: client,
    }
}
```

---

## 6. 完整使用流程

### 6.1 模块注册 Hook（示例：代理模块）

```go
// proxy/init.go
package proxy

import (
    "nofx/hook"
    "github.com/adshao/go-binance/v2/futures"
)

// InitHooks 初始化代理模块的 Hooks
func InitHooks(enabled bool) {
    if !enabled {
        return  // 条件不满足，不注册
    }

    // 1. 注册 IP 获取 Hook
    hook.RegisterHook(hook.GETIP, func(args ...any) any {
        userId := args[0].(string)
        
        // 获取用户专用代理 IP
        proxyIP, err := getProxyIP(userId)
        
        return &hook.IpResult{
            Err: err,
            IP:  proxyIP,
        }
    })

    // 2. 注册 Binance 客户端 Hook
    hook.RegisterHook(hook.NEW_BINANCE_TRADER, func(args ...any) any {
        userId := args[0].(string)
        client := args[1].(*futures.Client)

        // 修改客户端配置（添加代理）
        if client.HTTPClient != nil {
            client.HTTPClient.Transport = &http.Transport{
                Proxy: http.ProxyURL(getProxyURL(userId)),
            }
        }

        return &hook.NewBinanceTraderResult{
            Client: client,
        }
    })

    // 3. 注册 Aster 客户端 Hook
    hook.RegisterHook(hook.NEW_ASTER_TRADER, func(args ...any) any {
        user := args[0].(string)
        client := args[1].(*http.Client)

        // 设置代理
        if transport, ok := client.Transport.(*http.Transport); ok {
            transport.Proxy = http.ProxyURL(getProxyURL(user))
        }

        return &hook.NewAsterTraderResult{
            Client: client,
        }
    })

    // 4. 注册 HTTP 客户端 Hook
    hook.RegisterHook(hook.SET_HTTP_CLIENT, func(args ...any) any {
        client := args[0].(*http.Client)

        // 设置代理
        client.Transport = &http.Transport{
            Proxy: http.ProxyURL(getDefaultProxyURL()),
        }

        return &hook.SetHttpClientResult{
            Client: client,
        }
    })
}
```

### 6.2 应用启动时初始化

```go
// main.go
package main

import (
    "log"
    "nofx/config"
    "nofx/proxy"
)

func main() {
    // 1. 加载配置
    cfg := loadConfig()
    
    // 2. 初始化代理模块（注册 Hooks）
    proxy.InitHooks(cfg.Proxy.Enabled)
    
    // 3. 启动应用（Hooks 已注册，可以正常使用）
    if err := startServer(); err != nil {
        log.Fatal(err)
    }
}
```

### 6.3 业务代码调用 Hook

```go
// api/server.go
func (s *Server) handleGetServerIP(c *gin.Context) {
    userId := c.GetString("user_id")
    
    // 调用 Hook
    userIP := hook.HookExec[hook.IpResult](hook.GETIP, userId)
    
    // 检查结果
    if userIP != nil && userIP.Error() == nil {
        // 使用 Hook 返回的 IP
        c.JSON(http.StatusOK, gin.H{
            "public_ip": userIP.GetResult(),
            "message":   "请将此IP地址添加到白名单中",
        })
        return
    }
    
    // Fallback：使用默认逻辑
    publicIP := getPublicIPFromAPI()
    if publicIP == "" {
        publicIP = getPublicIPFromInterface()
    }
    
    if publicIP == "" {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "无法获取公网IP地址",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "public_ip": publicIP,
        "message":   "请将此IP地址添加到白名单中",
    })
}
```

### 6.4 完整流程图

```
┌─────────────────────────────────────────────────────────┐
│                    应用启动流程                           │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  1. main() 函数执行                                       │
│     │                                                     │
│     ├─> 加载配置 (config.Load())                         │
│     │                                                     │
│     ├─> proxy.InitHooks(cfg.Proxy.Enabled)              │
│     │     │                                               │
│     │     ├─> hook.RegisterHook(GETIP, ...)             │
│     │     ├─> hook.RegisterHook(NEW_BINANCE_TRADER, ...) │
│     │     └─> hook.RegisterHook(NEW_ASTER_TRADER, ...)   │
│     │                                                     │
│     └─> startServer()                                    │
│                                                           │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│                    运行时调用流程                         │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  1. 业务代码需要获取用户 IP                               │
│     │                                                     │
│     ├─> hook.HookExec[IpResult](GETIP, userId)          │
│     │     │                                               │
│     │     ├─> 检查 EnableHooks                           │
│     │     ├─> 查找 Hooks[GETIP]                          │
│     │     ├─> 执行 Hook 函数                             │
│     │     └─> 返回 *IpResult                             │
│     │                                                     │
│     ├─> 检查 result != nil && result.Error() == nil     │
│     │     │                                               │
│     │     ├─> Yes: 使用 Hook 返回的 IP                   │
│     │     └─> No: 使用 Fallback 逻辑                     │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

---

## 7. 设计特点

### 7.1 类型安全

使用 Go 泛型确保类型安全：

```go
// 编译时类型检查
result := hook.HookExec[hook.IpResult](hook.GETIP, userId)
// result 的类型是 *hook.IpResult，IDE 可以提供自动补全

// 如果类型不匹配，编译时会报错
result := hook.HookExec[hook.IpResult](hook.NEW_BINANCE_TRADER, userId, client)
// ❌ 编译错误：类型不匹配
```

### 7.2 自动 Fallback

Hook 未注册时返回 `nil`，业务代码可以优雅地回退到默认逻辑：

```go
result := hook.HookExec[hook.IpResult](hook.GETIP, userId)
if result != nil && result.Error() == nil {
    // 使用 Hook 结果
    return result.GetResult()
}

// Fallback：使用默认逻辑
return getDefaultIP()
```

**优势**:
- 核心代码不依赖 Hook 实现
- Hook 是可选的，不影响主流程
- 便于测试（可以不注册 Hook）

### 7.3 错误处理

每个 Result 类型都有 `Error()` 方法，统一错误处理：

```go
type IpResult struct {
    Err error
    IP  string
}

func (r *IpResult) Error() error {
    if r.Err != nil {
        log.Printf("⚠️ 执行GetIP时出错: %v", r.Err)
    }
    return r.Err
}
```

**使用方式**:

```go
result := hook.HookExec[hook.IpResult](hook.GETIP, userId)
if result != nil && result.Error() == nil {
    // 成功，使用结果
    ip := result.GetResult()
} else if result != nil {
    // Hook 执行失败，记录错误
    log.Printf("Hook 执行失败: %v", result.Error())
    // 使用 Fallback
}
```

### 7.4 日志记录

执行时自动记录日志，便于调试：

- `🔌 Execute hook: {KEY}` - Hook 存在并执行
- `🔌 Do not find hook: {KEY}` - Hook 未注册
- `🔌 Hooks are disabled, skip hook: {KEY}` - Hook 被禁用

**示例日志输出**:

```
🔌 Execute hook: GETIP
🔌 Execute hook: NEW_BINANCE_TRADER
🔌 Do not find hook: CUSTOM_HOOK
```

### 7.5 全局开关

可以通过 `EnableHooks` 全局禁用所有 Hook：

```go
// 禁用所有 Hook（用于调试或紧急情况）
hook.EnableHooks = false

// 所有 HookExec 调用都会返回 nil，不会执行任何 Hook
```

---

## 8. 最佳实践

### ✅ 推荐做法

#### 1. 在注册时判断条件

```go
func InitHooks(enabled bool) {
    if !enabled {
        return  // 不注册，避免不必要的开销
    }
    
    hook.RegisterHook(hook.GETIP, myHookFunc)
}
```

**优势**: 
- 避免注册不必要的 Hook
- 减少内存占用
- 提高性能（不需要在运行时判断）

#### 2. 总是返回正确的 Result 类型

```go
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    ip, err := getIP()
    
    // ✅ 总是返回 Result 类型，即使出错
    return &hook.IpResult{
        Err: err,
        IP:  ip,
    }
})
```

**优势**:
- 统一的错误处理
- 调用方可以检查错误
- 类型安全

#### 3. 安全的类型断言

```go
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    // ✅ 使用类型断言检查
    userId, ok := args[0].(string)
    if !ok {
        return &hook.IpResult{
            Err: fmt.Errorf("参数类型错误：期望 string，得到 %T", args[0]),
        }
    }
    
    // 使用 userId...
})
```

**优势**:
- 避免 panic
- 提供清晰的错误信息
- 提高代码健壮性

#### 4. 在模块初始化函数中注册

```go
// proxy/init.go
package proxy

func init() {
    // 在 init() 中注册，确保在应用启动前完成
    hook.RegisterHook(hook.GETIP, myHookFunc)
}
```

**或者使用显式初始化函数**:

```go
// proxy/init.go
package proxy

func InitHooks(cfg *Config) {
    if !cfg.Enabled {
        return
    }
    
    hook.RegisterHook(hook.GETIP, myHookFunc)
}

// main.go
func main() {
    cfg := loadConfig()
    proxy.InitHooks(cfg.Proxy)  // 显式调用
}
```

### ❌ 避免的做法

#### 1. 不要在 Hook 内部判断条件

```go
// ❌ 不推荐
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    if !enabled {  // 每次调用都判断，浪费性能
        return nil
    }
    // ...
})

// ✅ 推荐
func InitHooks(enabled bool) {
    if !enabled {
        return  // 注册时判断，只判断一次
    }
    hook.RegisterHook(hook.GETIP, myHookFunc)
}
```

#### 2. 不要直接 panic

```go
// ❌ 不推荐
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    userId := args[0].(string)  // 如果类型不匹配会 panic
    // ...
})

// ✅ 推荐
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    userId, ok := args[0].(string)
    if !ok {
        return &hook.IpResult{
            Err: fmt.Errorf("参数类型错误"),
        }
    }
    // ...
})
```

#### 3. 不要跳过错误检查

```go
// ❌ 不推荐
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    ip, _ := getIP()  // 忽略错误
    return &hook.IpResult{IP: ip}
})

// ✅ 推荐
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    ip, err := getIP()
    return &hook.IpResult{
        Err: err,
        IP:  ip,
    }
})
```

#### 4. 不要在 Hook 中执行耗时操作

```go
// ❌ 不推荐（如果操作很耗时）
hook.RegisterHook(hook.GETIP, func(args ...any) any {
    // 同步调用外部 API，可能阻塞
    ip := callSlowAPI()
    return &hook.IpResult{IP: ip}
})

// ✅ 推荐（使用缓存或异步）
var ipCache = make(map[string]string)

hook.RegisterHook(hook.GETIP, func(args ...any) any {
    userId := args[0].(string)
    
    // 使用缓存
    if ip, ok := ipCache[userId]; ok {
        return &hook.IpResult{IP: ip}
    }
    
    // 或者异步获取
    ip := getCachedIP(userId)
    return &hook.IpResult{IP: ip}
})
```

---

## 9. 添加新 Hook 扩展点

### 步骤 1: 定义 Result 类型

创建新文件 `hook/my_hook.go`:

```go
package hook

import "log"

type MyHookResult struct {
    Err  error
    Data string
}

func (r *MyHookResult) Error() error {
    if r.Err != nil {
        log.Printf("⚠️ Hook出错: %v", r.Err)
    }
    return r.Err
}

func (r *MyHookResult) GetResult() string {
    r.Error()
    return r.Data
}
```

### 步骤 2: 定义 Hook 常量

在 `hook/hooks.go` 中添加常量:

```go
const (
    GETIP              = "GETIP"
    NEW_BINANCE_TRADER = "NEW_BINANCE_TRADER"
    NEW_ASTER_TRADER   = "NEW_ASTER_TRADER"
    SET_HTTP_CLIENT    = "SET_HTTP_CLIENT"
    MY_HOOK            = "MY_HOOK"  // 新增
)
```

### 步骤 3: 在业务代码中调用

在需要的地方调用 Hook:

```go
// some_module.go
import "nofx/hook"

func someFunction() {
    result := hook.HookExec[hook.MyHookResult](
        hook.MY_HOOK, 
        arg1, 
        arg2,
    )
    
    if result != nil && result.Error() == nil {
        data := result.GetResult()
        // 使用 data
    } else {
        // Fallback 逻辑
    }
}
```

### 步骤 4: 注册实现

在模块的初始化函数中注册:

```go
// my_module/init.go
package my_module

import "nofx/hook"

func InitHooks(enabled bool) {
    if !enabled {
        return
    }
    
    hook.RegisterHook(hook.MY_HOOK, func(args ...any) any {
        arg1 := args[0].(string)
        arg2 := args[1].(int)
        
        // 处理逻辑
        data, err := processData(arg1, arg2)
        
        return &hook.MyHookResult{
            Err:  err,
            Data: data,
        }
    })
}
```

### 步骤 5: 更新文档

在 `hook/README.md` 中添加新 Hook 的文档:

```markdown
### 5. `MY_HOOK` - 我的自定义 Hook

**调用位置**：`some_module.go:XX`

**参数**：`arg1 string, arg2 int`

**返回**：`*MyHookResult`

**用途**：描述 Hook 的用途
```

---

## 10. 总结

### 核心优势

1. **解耦**: 核心代码不依赖具体实现，通过 Hook 注入逻辑
2. **灵活**: 支持动态注册和配置，适应不同环境需求
3. **安全**: 类型安全的泛型 API，编译时检查
4. **健壮**: 自动 Fallback，Hook 失败不影响主流程
5. **可维护**: 清晰的扩展点，便于添加新功能

### 适用场景

- ✅ **代理配置**: 为不同用户或环境设置不同的代理
- ✅ **日志记录**: 添加自定义日志记录逻辑
- ✅ **监控**: 注入监控和追踪代码
- ✅ **测试**: 在测试中模拟或替换实现
- ✅ **环境适配**: 根据环境（开发/生产）注入不同逻辑

### 注意事项

1. **性能**: Hook 会在关键路径上执行，避免耗时操作
2. **错误处理**: 总是返回正确的 Result 类型，包含错误信息
3. **类型安全**: 使用类型断言检查参数类型
4. **文档**: 为新 Hook 添加清晰的文档说明

### 相关文件

- **核心实现**: `hook/hooks.go`
- **Result 类型**: 
  - `hook/ip_hook.go`
  - `hook/trader_hook.go`
  - `hook/http_client_hook.go`
- **调用示例**: 
  - `api/server.go` (GETIP)
  - `trader/binance_futures.go` (NEW_BINANCE_TRADER)
  - `trader/aster_trader.go` (NEW_ASTER_TRADER)
  - `market/api_client.go` (SET_HTTP_CLIENT)
- **使用文档**: `hook/README.md`

---

**最后更新**: 2024年

**维护者**: 开发团队

