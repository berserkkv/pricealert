# Crypto Price Alerts

A simple Go app that monitors Binance USDT-M futures prices and sends Telegram notifications when horizontal or channel alerts trigger. Includes an embedded web UI (HTML/CSS/JS) served from the binary.

## Features

- **Horizontal alerts** — notify when price goes above or below a target
- **Channel alerts** — diagonal parallel channel from two datetime/price points plus offset
- **Web UI** — manage alerts in a single-page table (create, edit, delete, enable/disable)
- **Telegram** — optional notifications via Bot API
- **JSON storage** — alerts saved to `alerts.json` (auto-save on every change)
- **No extra dependencies** — standard library only

## Requirements

- Go 1.22 or newer
- Network access to Binance Futures API (`fapi.binance.com`) and (optionally) Telegram

## Install

```bash
sudo systemctl stop pricealert 2>/dev/null || true && curl -fsSL https://raw.githubusercontent.com/berserkkv/pricealert/main/install.sh | sudo bash
```

## Build

```bash
go build -o pricealert
```

To run without building a binary (must pass the whole package, not a single file):

```bash
go run .
```

Do **not** use `go run main.go` — that only compiles `main.go` and will fail with undefined symbols.

## Run

1. Copy the example config (optional):
  ```bash
   cp config.json.example config.json
  ```
2. Edit `config.json` — set `telegram_token` and `telegram_chat_id` for notifications.
3. Start the app:
  ```bash
   ./price-alert
  ```
4. Open the UI: [http://localhost:8080](http://localhost:8080)

## Configuration

`config.json` overrides built-in defaults. Missing keys keep defaults.


| Key                       | Default           | Description                                            |
| ------------------------- | ----------------- | ------------------------------------------------------ |
| `http_port`               | `8080`            | Web server port                                        |
| `poll_interval_sec`       | `5`               | Binance price poll interval                            |
| `binance_symbol`          | `SOLUSDT`         | Default futures symbol (alerts can set their own pair) |
| `telegram_token`          | `""`              | Telegram bot token                                     |
| `telegram_chat_id`        | `""`              | Telegram chat ID                                       |
| `storage_file`            | `alerts.json`     | Alert persistence file                                 |
| `touch_tolerance_percent` | `0.15`            | Channel touch tolerance (±%)                           |
| `utc`                     | `Europe/Istanbul` | IANA timezone for diagonal channel datetimes           |


## Market data

Prices come from **Binance USDT-M Futures** (no API key required):

`GET https://fapi.binance.com/fapi/v1/ticker/price?symbol=SOLUSDT`

Use the same symbol names as on futures (e.g. `SOLUSDT`, `BTCUSDT`).

## Timezone (diagonal channel)

Set `utc` in `config.json` to an [IANA timezone](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) name. Default is `Europe/Istanbul` (UTC+3).

All channel datetime inputs and calculations use this timezone:

- UI datetime pickers show/edit times in this zone
- API parses `datetime-local` values as wall time in this zone
- Channel line evaluation uses the current instant in this zone

Example:

```json
"utc": "Europe/Istanbul"
```

Match the same timezone you use on TradingView charts.

## Channel math

Two points define a line using unix time as X and price as Y:

- slope `m = (y2 - y1) / (x2 - x1)`
- intercept `b = y1 - m * x1`
- base line at time `t`: `price = m * t + b`
- parallel line: `price = m * t + b + offset`
- upper/lower boundaries are the max/min of the two lines at the current time

An alert fires when market price enters the tolerance band around the chosen boundary (upper, lower, or both).

## REST API


| Method | Path                      | Description     |
| ------ | ------------------------- | --------------- |
| GET    | `/api/alerts`             | List all alerts |
| POST   | `/api/alerts`             | Create alert    |
| PUT    | `/api/alerts/{id}`        | Update alert    |
| DELETE | `/api/alerts/{id}`        | Delete alert    |
| POST   | `/api/alerts/{id}/toggle` | Enable/disable  |


## Ntfy (local, no ntfy.sh)

Pricealert embeds a small [ntfy](https://ntfy.sh)-compatible server on port **8090** (configurable via `ntfy_port`). Alerts are published only to your machine; nothing is sent to public ntfy.sh.

1. In `config.json`, set `ntfy_enabled`: true and `ntfy_topic` (e.g. `price_alerts1412`).
2. Restart the app. Logs show: `ntfy (local) http://0.0.0.0:8090`.

### Subscribe on Android (ntfy app)

1. Install [ntfy for Android](https://play.google.com/store/apps/details?id=io.heckel.ntfy) (or F-Droid).
2. Find your PC/server **LAN IP** (e.g. `192.168.1.50`) — the phone must be on the same Wi‑Fi.
3. In the ntfy app: **⋮ → Settings → Default server** → enter:
   ```
   http://192.168.1.50:8090
   ```
   (Use your real IP and port if you changed `ntfy_port`.)
4. Tap **+** to add a subscription and enter the **same topic** as in config, e.g. `price_alerts1412`.
5. Allow notifications when prompted.

**Note:** HTTP (not HTTPS) on a private LAN is normal for a home server. If the app refuses cleartext HTTP, use a reverse proxy with TLS or check the app’s “allow insecure connection” option for custom servers.

Test from another machine on the LAN:

```bash
curl -d "test" -H "Title: Hello" http://192.168.1.50:8090/price_alerts1412
```

| Key            | Default             | Description                          |
| -------------- | ------------------- | ------------------------------------ |
| `ntfy_enabled` | `false`             | Send alerts via local ntfy           |
| `ntfy_topic`   | `price_alerts1412`  | Topic name (subscribe to this in app)|
| `ntfy_port`    | `8090`              | Local ntfy server port               |

## Telegram setup

1. Create a bot via [@BotFather](https://t.me/BotFather) and copy the token.
2. Get your chat ID (e.g. message [@userinfobot](https://t.me/userinfobot) or use the Bot API `getUpdates`).
3. Add both values to `config.json`.

## Project layout

```
main.go
config.go
models.go
storage.go
telegram.go
checker.go
web.go
templates/index.html
static/app.js
static/style.css
config.json.example
README.md
```

## License

Use and modify freely.