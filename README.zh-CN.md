# EZ VPN Lego

[Русский](README.md) | [English](README.en.md) | [简体中文](README.zh-CN.md)

在自己的服务器上搭建 VPN。安装后绑定 Telegram，即可创建连接凭据。
无需强制安装管理面板，也不依赖 Lego 的托管服务。

已发布的版本支持单台 VPS，使用 VLESS REALITY Vision，通过 Telegram 管理。
模块化版本正在开发中，将支持直连和经过入口节点、出口节点的两节点线路。

## 目前可用

- 支持全新安装的 Ubuntu Server 24.04，`amd64` 和 `arm64`；
- 通过一次性链接绑定 Telegram 管理员；
- 为不同用户和设备创建独立凭据；
- 应用配置前进行校验，应用失败时回滚；
- 每日备份和经过签名的更新；
- Telegram 或控制服务不可用时，已有 VPN 连接仍可继续工作。

## 安装已发布的版本

准备一台全新的 VPS，并通过 [@BotFather](https://t.me/BotFather) 创建自己的机器人。
在服务器上运行：

```bash
curl -fsSL https://raw.githubusercontent.com/steam9steam-prog/ez-vpn-lego/main/install.sh | sudo bash
```

安装程序会要求输入机器人令牌，输入内容不会显示。
随后会显示一次性 Telegram 绑定链接和二维码，链接在 15 分钟后失效。

此模式不要求自有域名。服务器的 TCP 443 端口必须空闲，并可从公网访问。
不要覆盖正在运行的其他服务。

## 开发中的模块化版本

```text
用户 → VPN 节点 → 互联网
用户 → 入口节点 → 出口节点 → 互联网
```

入口和出口不限国家。控制服务可以与入口或出口共用服务器，也可以单独部署。
数据模型不设置固定的节点数量上限。

本分支已包含拓扑规划器、Xray 配置生成器和本地准备工具。
工具会保留已有密钥、生成独立订阅文件，并检查 DNS。
规划器可通过统一 API 使用，Telegram 可显示连接配置目录。

远程节点接入和完整的 Telegram 多节点管理尚未完成。
公开安装脚本目前仍安装单节点版本。生成文件不代表已将配置应用到服务器。

- [试用模块化工具](docs/zh-CN/modular.md)
- [域名和证书](docs/zh-CN/domains.md)
- [首个模块化版本的范围](docs/product-requirements.md)，英文
- [组件架构](docs/architecture.md)，英文

## 维护单节点安装

```bash
sudo -u ezvpn bash -c 'set -a; source /etc/ez-vpn-lego/ezvpn.env; set +a; lego-vpnctl status'
sudo ez-vpn-lego backup
sudo ez-vpn-lego update
sudo ez-vpn-lego restore /path/to/backup.tar.gz
```

备份包含私钥。请妥善保存，并复制到 VPS 以外的安全位置。
当前生成的备份压缩包本身未加密。

更多信息见 [维护说明](docs/operations.md)、[威胁模型](docs/threat-model.md)
和 [API 文档](api/openapi.yaml)，目前为英文。

## 开发和测试

```bash
make generate-check
make fmt vet test-race build
XRAY_BINARY=/absolute/path/to/xray go test -race ./drivers/xray/topology -v
```

Xray 测试会在本机回环网络中验证真实的直连和两节点线路。
安装程序另有一次性 Ubuntu 虚拟机测试：

```bash
./tests/e2e/ubuntu-vm.sh
```

## 许可证

[GNU AGPL-3.0](LICENSE)。
