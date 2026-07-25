# Windows 客户端版本发布

本文是服务端版本管理员的操作手册。客户端打包和更新器内部实现见 jdShop 项目的 `docs/windows-portable-release.md` 与 `docs/client-update-system.md`。

## 1. 架构边界

- jdShopServer只保存版本码、说明、下载地址、文件大小、SHA-256和强制更新标志；
- 约134MB的Windows ZIP直接由阿里云OSS等对象存储下载；
- 不通过12Mbps业务服务器转发安装包；
- 客户端从 `GET /api/v1/version/latest` 获取元数据，再由独立更新器下载和安装。

## 2. 当前生产版本

```text
version_name: 0.2.6
version_code: 2026072502
platform: windows
arch: win-x64
```

当前构建记录：

| 字段 | 值 |
|------|----|
| 文件名 | `JDMonitor-0.2.6-win-x64.zip` |
| 文件大小 | `135163057` 字节 |
| SHA-256 | `410dd15fde0c33e8837767828f1ec6dbad9fedcc92fe9e502e6514a793b2c9eb` |

注意：重新构建同一版本会改变文件大小和哈希。后台登记必须以最终上传且不再修改的OSS对象为准，不能盲目复制本文数值。

## 3. 构建客户端

在 `G:\jdShop` 设置生产API并构建：

```powershell
chcp 65001
$env:JDSHOP_API_BASE_URL = "https://api.jdshop.bbroot.com/api/v1"
powershell -ExecutionPolicy Bypass -File .\scripts\build_portable.ps1
```

v0.2.6 起，便携版必须将 `certifi/cacert.pem` 打包进启动器、服务和独立更新器三个 EXE。它们同时保留 Windows 系统根证书，并通过严格 TLS 上下文访问登录、版本查询和更新服务。发布前应使用 `pyi-archive_viewer` 检查三个 EXE 均包含：

```text
certifi\cacert.pem
```

不得通过关闭证书校验来规避问题。如果客户网络被企业代理或杀毒软件替换为自签名 HTTPS 证书，仍需在客户电脑中安装对应的企业根证书。

构建结果：

```text
release/JDMonitor-{version}-win-x64/
release/JDMonitor-{version}-win-x64.zip
release/update-feed-{version}.json
```

版本名称用于用户展示；`version_code`用于更新比较，必须严格大于此前发布的所有Windows版本码。

## 4. 上传到阿里云OSS

推荐对象名：

```text
JDMonitor-0.2.6-win-x64.zip
```

规则：

1. 文件名包含版本号；
2. 发布后不覆盖同名对象；
3. 从OSS重新下载一次，复核大小和SHA-256；
4. 在主要客户网络实测速度和稳定性；
5. 保留至少上一个稳定版本的对象；
6. 不把源码、数据库、Cookie、浏览器Profile、日志、AI Key或客户账号数据上传到发布Bucket。

### 4.1 下载地址必须长期有效

不要把以下类型的临时签名URL登记到后台：

```text
https://bucket.oss-.../file.zip?Expires=...&OSSAccessKeyId=TMP...&Signature=...
```

它很长是因为包含到期时间、临时访问密钥和签名；到期后已发布客户端会全部下载失败。

公开客户端发布包应使用不含查询参数的永久HTTPS地址，例如：

```text
https://jdshop-client-releases-hk.oss-cn-hongkong.aliyuncs.com/JDMonitor-0.2.6-win-x64.zip
```

如果Bucket保持私有，则需要长期稳定的下载网关按请求生成新签名，不能把控制台一次性签名写入版本表。最简单的第一阶段方案是只对发布ZIP对象开放公共读，且Bucket内不存放私密资料。

## 5. 复核最终对象

Windows下载后：

```powershell
chcp 65001
$file = 'JDMonitor-0.2.6-win-x64.zip'
(Get-Item -LiteralPath $file).Length
(Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant()
```

也可检查HTTP响应：

```powershell
Invoke-WebRequest -Uri '永久OSS地址' -Method Head
```

必须确认HTTPS证书正常、状态200、`Content-Length`与后台准备登记的大小一致。

## 6. 管理台登记

登录：

```text
https://www.jdshop.bbroot.com/admin
```

进入“版本管理”，填写：

| 字段 | 要求 |
|------|------|
| 平台 | `windows` |
| 版本码 | `update-feed`中的严格递增整数 |
| 版本名称 | 例如 `0.2.6` |
| 标题 | 一句话概括用户可感知变化 |
| 更新说明 | 分行说明新增、修复和注意事项 |
| 下载地址 | OSS永久HTTPS地址，不含临时签名参数 |
| 文件大小 | 最终OSS对象的字节数 |
| SHA-256 | 最终OSS对象的64位小写哈希，可带或不带 `sha256:` 前缀 |
| 强制更新 | 仅用于安全、协议或数据兼容性要求旧版必须退出的情况 |

创建新版本后会成为该平台最新版本。发布前再次确认下载地址、大小和哈希；错一个字符都会导致所有客户端拒绝安装。

## 7. 强制更新说明

0.2.3启动器支持不可取消、不可关闭的强制更新窗口。设置强制更新后，支持该逻辑的旧客户端只能点击“立即更新”，更新完成前不能继续使用旧程序。

但“是否允许取消”由客户电脑上已经安装的启动器实现。早于强制更新功能的客户端第一次升级时，服务端无法远程重写它的界面行为。因此：

- 从0.2.3起，后续版本可依赖新的强制更新窗口；
- 对更早版本仍需保留兼容期、账号控制或人工通知；
- 不应把强制更新当作撤回已泄露包或即时远程删除客户端的唯一机制。

## 8. 客户端安装体验

新版独立更新器显示：准备、下载、大小/SHA-256校验、解压、停止旧服务、备份程序、备份数据库、安装文件、启动健康检查、完成/回滚。

下载阶段显示百分比、已下载大小和总大小；错误详情写入：

```text
runtime/logs/updater.log
```

更新器保留 `runtime/`、浏览器Profile、SQLite数据库、登录状态、AI Key和 `config/client.json`。新服务健康检查失败时恢复程序和数据库备份。

## 9. 发布前检查

1. 解压发布ZIP，确认包含启动器、服务、更新器、`static/`、`manifest.json` 和面向客户的 UTF-8 `更新说明.txt`。
2. 确认 `manifest.json` 的版本名称和版本码正确。
3. 用临时端口启动发布版并确认 `/api/health` 返回新版本。
4. 用 `pyi-archive_viewer` 确认三个 EXE 均包含 `certifi\cacert.pem`。
5. 确认发布目录没有开发数据库、浏览器Profile、Cookie、日志和个人配置。
6. 上传OSS后重新下载，复核字节数和SHA-256。
7. 使用永久HTTPS地址，不使用带 `Expires` 的链接。
8. 在管理台核对所有版本字段后再发布。

## 10. 发布后测试矩阵

| 场景 | 预期 |
|------|------|
| 低版本检查 | 收到0.2.6元数据和正确说明 |
| 同版本检查 | `has_update=false`，不重复提示 |
| 普通更新 | 可暂不更新，选择更新后显示完整进度 |
| 强制更新 | 新启动器不允许取消或关闭 |
| 下载正常 | 百分比和字节数持续变化 |
| 下载地址失效 | 明确报下载失败，旧版本文件保持不变 |
| 大小或哈希错误 | 拒绝安装并保留旧版本 |
| 健康检查失败 | 自动回滚并尝试重启旧服务 |
| 更新成功 | 数据库、浏览器登录、AI Key、jdShop账号状态和API地址均保留 |

测试API：

```bash
curl 'https://api.jdshop.bbroot.com/api/v1/version/latest?platform=windows&current_version_code=2026072302'
curl 'https://api.jdshop.bbroot.com/api/v1/version/latest?platform=windows&current_version_code=2026072501'
```

## 11. 回退版本

发现严重问题时：

1. 立即在版本管理中取消问题版本的最新状态，或发布一个版本码更高的修复包；
2. 不要用旧ZIP覆盖问题版本的同名OSS对象；
3. 如果客户端已安装问题版本，单纯把低版本设为最新通常不会触发降级，因为客户端只接受更大的版本码；
4. 需要自动回退时，应以更高版本码发布包含旧稳定代码的修复包；
5. 保留问题包、版本元数据和客户端更新日志用于复盘。

## 12. 当前安全边界

当前使用HTTPS、文件大小和SHA-256，可防止传输损坏和下载文件与后台登记不一致。下一阶段应增加离线私钥签名的更新清单和Windows Authenticode签名，降低版本服务或对象存储元数据同时被篡改的风险。
