

# Triangle Detector

Detects ascending triangle chart patterns in crypto candlestick data from Binance.

## Structure

```
cmd/triangled/      — main binary (batch scan + realtime monitor)
internal/
  domain/           — shared types (Candle)
  detect/           — ascending triangle detection pipeline
  render/           — ChartRenderer interface
  render/echarts/   — go-echarts chart renderer
  app/              — render orchestration facade
  artifact/         — output file path management
  screenshot/       — chromedp HTML→PNG conversion
  marketdata/binance/ — Binance REST API client
  config/           — .env loading, AppConfig
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full package dependency graph.

## Build

```sh
go build ./cmd/triangled/
# or via Makefile:
make build
```

## Usage

Copy `.env.example` to `.env` and set `DATA_DIR` and `SYMBOLS`:

```env
DATA_DIR=tmp
SYMBOLS=BTCUSDT,ETHUSDT,BNBUSDT
```

**Batch scan** (one pass over all symbols):
```sh
./triangled
```

**Realtime monitor** (continuous polling):
```sh
./triangled -realtime
```

## Tests

```sh
make test
# or directly:
go test ./internal/...
```

### Contact

- Telegram: @gof4rvr
- Email: r3ndyhell@gmail.com

