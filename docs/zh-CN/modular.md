# 试用模块化工具

[Русский](../ru/modular.md) | [English](../en/modular.md) | [简体中文](../zh-CN/modular.md)

这是本地准备工具，不会连接 VPS，也不会修改正在运行的 VPN。
远程应用配置和完整的 Telegram 多节点管理属于下一阶段。

## 构建

需要 Go 1.26.9。在仓库根目录运行：

```bash
mkdir -p bin workbench
go build -o bin/lego-vpnctl ./cmd/lego-vpnctl
cp examples/cascade.json workbench/installation.json
bin/lego-vpnctl topology plan workbench/installation.json
```

示例包含一个入口和三个出口。示例地址仅用于文档，不能作为真实服务器地址。
`examples` 中还提供 `direct.json`、`separate-control.json` 和 `xhttp-tls.json`。

## 节点和线路

- `control` 管理部署，本身不代表一个流量转发节点。
- `entry` 接收用户连接。
- `exit` 将两节点线路的流量发送到互联网。

一台服务器可以承担多个角色。每套部署恰好有一个控制节点。
入口和出口没有固定数量上限。国家字段只用于显示，不决定路由。

每条线路有 `entry_id`，以及可选的 `exit_id`。不填出口表示直连；
填写出口表示两节点线路。不支持更长的转发链。

如果连接配置和域名相同，多条线路可以共用入口端口。不同凭据选择不同出口。
出口不可用时，不会自动改为入口直接出网。节点之间默认使用 TCP 10443 上的
REALITY，可以通过出口节点的 `relay_port` 修改端口。

可选的 `reality_target`（`host:port`）和 `reality_server_name` 必须一起填写。
默认目标是 `www.microsoft.com:443`，需要在自己的网络中另行验证。

当前请使用 `owner: lego`。`remnawave` 和 `marzban` 为后续适配器预留。
在适配器完成前，规划器会拒绝这些节点，不会直接接管面板的配置。

## 创建密钥和连接凭据

```bash
bin/lego-vpnctl topology init workbench/installation.json workbench/state.json
bin/lego-vpnctl topology grant workbench/installation.json workbench/state.json phone "My phone" uk us ca
bin/lego-vpnctl topology render workbench/installation.json workbench/state.json workbench/generation-1
```

节点配置位于 `generation-1/nodes`，订阅文件位于 `generation-1/subscriptions`，
校验值位于 `manifest.json`。命令输出不会打印密钥。每条获准线路使用独立凭据。

`state.json` 包含私钥，权限为 0600；生成目录权限为 0700。
这是本地工具的状态文件，尚未加密，并非生产控制服务的存储。
不要公开这些文件，也不要通过网站发布整个目录。Git 已忽略 `workbench`。

订阅文件名是随机令牌，重新生成时保持不变。只能通过另行配置的 HTTPS 服务
发布 `subscriptions` 目录，并关闭目录列表和缓存。工具不会启动该服务。
公开 `nodes` 目录会泄露私钥。

## 扩展部署

修改部署描述后运行：

```bash
bin/lego-vpnctl topology extend workbench/installation.json workbench/state.json
bin/lego-vpnctl topology render workbench/installation.json workbench/state.json workbench/generation-2
```

已有密钥会保留。新增线路不会自动授权给已有设备。可以更新设备的线路列表，
保留仍获授权线路的凭据和原订阅令牌：

```bash
bin/lego-vpnctl topology routes workbench/installation.json workbench/state.json phone uk us ca
```

从列表中移除的线路不会出现在下一次生成的设备配置中。工具拒绝重新初始化已有状态文件，每次生成也必须使用新目录。

## 撤销或恢复访问

```bash
bin/lego-vpnctl topology device workbench/state.json phone disable
bin/lego-vpnctl topology render workbench/installation.json workbench/state.json workbench/generation-3
```

新生成的文件会同时移除订阅和入口凭据。只有实际更新服务器配置后，之前导入
的凭据才会失效。使用 `enable` 并重新生成，可恢复原来的凭据和订阅令牌。

删除部署描述中的线路后，新配置和订阅中也会移除该线路。
旧密钥保留在私有状态中，供管理员明确选择回滚时使用。

## 连接配置和 DNS

```bash
bin/lego-vpnctl profiles list
bin/lego-vpnctl topology dns workbench/installation.json
```

REALITY Vision 已实现；XHTTP REALITY 和 XHTTP TLS 仍为实验状态。
TLS 需要域名和另行准备的证书，详见 [域名说明](domains.md)。

统一控制 API 提供 `GET /v1/profiles` 和 `POST /v1/topology/plan`。
规划结果不包含秘密信息，也不会应用配置。绑定管理员后，Telegram 可通过
`/profiles` 显示连接配置目录。

## 测试范围

自动测试覆盖密钥保留、撤销访问、端口冲突、控制节点位置，以及 1003 个节点
的规划。最后一项只验证数据模型，不代表已测得生产容量。

设置 `XRAY_BINARY` 后，集成测试会在本机回环网络启动真实 Xray 进程，验证
直连和两节点线路。不同的出口源地址可以发现错误的直接出网。

仍需在俄罗斯和中国的实际运营商网络、真实 VPS 和具体版本的移动客户端上
验证，并进行负载测试。当前目录不保证某个连接配置在全国所有网络中可用。
