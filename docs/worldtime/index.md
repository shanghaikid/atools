# worldtime

终端世界时钟 —— 在命令行实时显示全球 10 个主要城市的当前时间。

## 功能

- 每秒实时刷新显示
- 10 个默认城市覆盖全球主要时区
- 彩色终端输出
- 按 `Ctrl+C` 优雅退出

## 安装

```bash
cd tools/worldtime
make install
```

## 使用

```bash
worldtime
```

输出示例：

```
  🌍 World Time Clock
  ─────────────────────────────────────────────

  ⏰ Local (CST)          14:30:25  Sat, 15 Feb 2026  UTC+8

  ─────────────────────────────────────────────

  🕐 New York             01:30:25  Sat, 15 Feb  UTC-5
  🕐 London               06:30:25  Sat, 15 Feb  UTC+0
  🕐 Paris                07:30:25  Sat, 15 Feb  UTC+1
  🕐 Dubai                10:30:25  Sat, 15 Feb  UTC+4
  🕐 Mumbai               12:00:25  Sat, 15 Feb  UTC+5
  🕐 Singapore            14:30:25  Sat, 15 Feb  UTC+8
  🕐 Shanghai             14:30:25  Sat, 15 Feb  UTC+8
  🕐 Tokyo                15:30:25  Sat, 15 Feb  UTC+9
  🕐 Sydney               17:30:25  Sat, 15 Feb  UTC+11
  🕐 Auckland             19:30:25  Sat, 15 Feb  UTC+13

  Press Ctrl+C to exit
```

## 默认城市

| 城市 | 时区 |
|------|------|
| New York | America/New_York |
| London | Europe/London |
| Paris | Europe/Paris |
| Dubai | Asia/Dubai |
| Mumbai | Asia/Kolkata |
| Singapore | Asia/Singapore |
| Shanghai | Asia/Shanghai |
| Tokyo | Asia/Tokyo |
| Sydney | Australia/Sydney |
| Auckland | Pacific/Auckland |

## 技术细节

- 纯 Go 标准库实现，无外部依赖
- 使用 `time.Ticker` 实现每秒刷新
- 使用 ANSI 转义码实现彩色输出和光标控制
- 通过 `os/signal` 监听 SIGINT/SIGTERM 实现优雅退出
