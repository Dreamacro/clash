<h1 align="center">
  <img src="https://github.com/Dreamacro/clash/raw/master/docs/logo.png" alt="Clash" width="200">
  <br>Clash<br>
</h1>

<h4 align="center">A rule-based tunnel in Go.</h4>

<p align="center">
  <a href="https://github.com/Dreamacro/clash/actions">
    <img src="https://img.shields.io/github/workflow/status/Dreamacro/clash/Go?style=flat-square" alt="Github Actions">
  </a>
  <a href="https://goreportcard.com/report/github.com/Dreamacro/clash">
    <img src="https://goreportcard.com/badge/github.com/Dreamacro/clash?style=flat-square">
  </a>
  <a href="https://github.com/Dreamacro/clash/releases">
    <img src="https://img.shields.io/github/release/Dreamacro/clash/all.svg?style=flat-square">
  </a>
</p>

## Features

- Local HTTP/HTTPS/SOCKS server with authentication support
- VMess, Shadowsocks, Trojan, Snell protocol support for remote connections
- Built-in DNS server that aims to minimize DNS pollution attack impact, supports DoH/DoT upstream and fake IP.
- Rules based off domains, GEOIP, IP CIDR or ports to forward packets to different nodes
- Remote groups allow users to implement powerful rules. Supports automatic fallback, load balancing or auto select node based off latency
- Remote providers, allowing users to get node lists remotely instead of hardcoding in config
- Netfilter TCP redirecting. Deploy Clash on your Internet gateway with `iptables`.
- Comprehensive HTTP RESTful API controller

## Getting Started
Documentations are now moved to [GitHub Wiki](https://github.com/Dreamacro/clash/wiki).

## Credits

* [riobard/go-shadowsocks2](https://github.com/riobard/go-shadowsocks2)
* [v2ray/v2ray-core](https://github.com/v2ray/v2ray-core)

## License

This software is released under the GPL-3.0 license.

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2FDreamacro%2Fclash.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2FDreamacro%2Fclash?ref=badge_large)

## TODO

- [x] Complementing the necessary rule operators
- [x] Redir proxy
- [x] UDP support
- [x] Connection manager
- [ ] ~~Event API~~
