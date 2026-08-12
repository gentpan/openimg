# Apple 客户端

macOS 桌面客户端（带窗口，非菜单栏），以及之后的 iOS 端。和后端同仓，不单独开仓库。

## 为什么在这个仓库里

`imageOut`（`backend/internal/api/image_handlers.go`）内嵌了整个 `models.Image`，
后端改一个字段客户端就要跟。分仓库时这类改动的表现是「后端合了、客户端三天后
崩」，而这个项目 61% 的历史提交本来就同时改动前后端。

## 结构

```
apple/OpenimgKit/
  Sources/OpenimgKit/     网络层、模型、钥匙串   ← 两端共用，无第三方依赖
  Sources/OpenimgMac/     macOS 应用（SwiftUI）
  Sources/KitCheck/       自检
```

## 跑起来

```bash
cd apple/OpenimgKit && swift run KitCheck
```

```bash
./apple/package-mac.sh release && open apple/build/OpenimgMac.app
```

**不要用 `swift run OpenimgMac`。** 裸可执行文件没有 bundle，`MenuBarExtra`
无处安放状态栏项，SwiftUI 找不到需要保活的 scene，进程 exit 0 就没了——不崩溃、
不打日志、菜单栏也没图标，看上去像什么都没发生。

首次运行用邮箱和密码登录。密码只用来换取一枚这台设备专用的长期令牌，不会被
保存；令牌进钥匙串，之后每次打开自动登录。也可以改用访问令牌登录（网站
「账号设置 → API Token」生成）。

## 几个刻意的选择

**自检是可执行目标，不是 test target。** XCTest 与 swift-testing 都要求完整
Xcode 才能解析——只装 Command Line Tools 时 `swift test` 根本编译不了，那样的
测试目标等于一堆没人跑过的断言。`swift run KitCheck` 在任何有 Swift 的地方都能
跑。等 Xcode 进场后可以再加一个 XCTest 目标，这个仍然留给 CI。

**Kit 收服务器地址，App 不收。** `OpenimgClient` 拿 URL 做参数——它是个库，
写死主机就没法测。但应用本身只面向 Openimg 一家，界面上不出现地址输入框，也
不显示完整网址：给用户一个输入框等于邀请他填一个不会工作的地址。将来要支持
自建实例时，Kit 这一层不用动。

`OpenimgClient.init` 仍然拒绝非回环地址的明文 http：令牌逐请求随 header 发送，
一个打错的域名就会把它交给应答的人。

**multipart 组装在磁盘上而不是内存里。** 后台 `URLSession` 只接受
`uploadTask(with:fromFile:)`，内存里的 body 会随进程退出一起消失。目前是前台
上传，但这条约束决定了组装方式，改起来比一开始就写对贵得多。

**视图里没有 `@State`。** 现行 SwiftUI 的 `@State` 是宏，宏插件同样需要完整
Xcode；只有 CLT 时整个 target 编译不过。那个拖放标志放进 model 也没有任何损失。

## 现状与下一步

Kit 与 macOS 应用都能用 `swift build` 构建，56 项自检全过。`package-mac.sh` 打出的是窗口化的真 .app，ad-hoc 签名，本机可用。
打包和 ad-hoc 签名都不需要 Xcode——.app 只是一个目录加 Info.plist，codesign
随 Command Line Tools 一起装。

**给别人用**才需要 Developer ID 签名加公证，那要 99 美元开发者账号。免费
Personal Team 签出来的 7 天过期且只能本机跑。

iOS 端还差一个产品决策：App Store 5.1.1(v) 要求提供账号功能的 App 必须能在
App 内删账号，而 `DELETE /api/account` 是 cookie-only，且 session 7 天硬过期、
没有刷新端点。这个边界是故意的——泄露的令牌不该能删号或再签发令牌——所以要么
iOS 端做 cookie 登录，要么完全不给登录入口（那又会撞 4.2 最低功能性）。

## 后端已具备的

令牌可达的接口（`backend/internal/api/router.go` 的 `machine` 组）：

```
POST   /api/upload             multipart，字段名固定 file，单文件
GET    /api/images             limit/offset/q/sort，返回 total
GET    /api/images/:id
DELETE /api/images/:id
POST   /api/images/bulk-delete
POST   /api/images/:id/variant
GET    /auth/me                当前令牌属于谁
GET    /api/quota              档位限制，用于上传前本地拦截
```

后两条是为原生端从 cookie-only 组移过来的。账号管理路由（改密码、删账号、
签发令牌）仍然只认 cookie。
