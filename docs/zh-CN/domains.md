# 域名

[Русский](../ru/domains.md) | [English](../en/domains.md) | [简体中文](../zh-CN/domains.md)

## 可以不使用自己的域名吗？

可以。VLESS REALITY Vision 和 XHTTP REALITY 可以直接使用服务器 IP。
REALITY 配置中会使用一个外部 TLS 网站的名称，但你无需拥有该域名。
需要确认节点能够访问所选的目标网站。

直连和两节点线路都可以这样使用。一个入口后面有三个出口，并不意味着需要
三个域名。节点之间的 REALITY 连接也可以使用 IP。

没有自有域名时，可以分享连接链接或二维码。Lego 计划中的公共 HTTPS
订阅功能使用由部署者控制的域名。

## 普通 TLS 和订阅

可以从任意注册商购买域名，也可以使用已有域名的子域名，例如
`vpn.example.com` 和 `sub.example.com`。Lego 不出售域名，也不要求使用
项目方统一提供的域名。

VPN 域名指向入口节点，而不是境外出口。订阅域名指向发布订阅的服务器，
它可以是单独的控制服务器。

IPv4 使用 A 记录；只有配置好 IPv6 后才添加 AAAA。基础 TLS 配置需要直接
解析，不经过 CDN 代理。在 Cloudflare 中请选择 DNS only。
请删除仍指向旧服务器的 AAAA 记录。

如果动态 DNS 服务允许申请所需证书，且域名持续可用，也可以使用动态 DNS。
系统不强制绑定某个 DNS 服务商。

## 当前工具可以检查什么？

```bash
lego-vpnctl topology plan installation.json
lego-vpnctl topology dns installation.json
```

规划器会列出 TLS 线路所需的域名。DNS 检查会比较全部 A 和 AAAA 记录。
如果节点同时使用 IPv4 和 IPv6，`address` 可以填写解析到这两个地址的主机名。

DNS 检查不等于申请证书，也不能证明某个运营商网络可用。
当前本地工具要求另外准备证书文件：
`/etc/ez-vpn-lego/certificates/域名/fullchain.pem` 和 `privkey.pem`。
私钥应允许 Xray 读取，但不能公开。自动申请和续期属于下一阶段的安装程序。

## 如果 443 端口已被占用

两个独立进程不能同时在同一个 IP 的 TCP 443 上提供 REALITY 和普通 HTTPS。
增加一个子域名也无法解决端口冲突。

计划中，同机部署且 REALITY 使用 443 时，HTTPS 订阅可以使用其他端口，
例如 8443。也可以使用另一个 IP，或在独立控制服务器上提供 HTTPS 443。
XHTTP/TLS 可以通过不同路径与订阅共用 HTTPS 入口，但该网关尚未实现。

安装程序应先解释方案并检查端口，再修改配置。不能覆盖现有网站或邮件 DNS。

## 所有权和恢复

域名注册和 DNS 始终由部署者管理。迁移服务器时，应通过修改 DNS 或订阅内容
保留个人订阅链接。紧急情况下仍需能通过 SSH 导出连接信息。
这些是模块化版本的目标，目前预览版尚未完成相关自动化。
