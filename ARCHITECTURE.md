# Architecture

## Package Dependency Graph

```
cmd/triangled/
    └── internal/config      (load .env, AppConfig)
    └── internal/domain      (Candle)
    └── internal/detect      (DetectAscendingTriangle, Result, Params, Option)
    └── internal/artifact    (Names, WriteTexts)
    └── internal/app         (RenderTriangleDetection facade)
    └── internal/render      (ChartRenderer interface)
    └── internal/render/echarts  (EChartsRenderer)
    └── internal/screenshot  (Screenshotter)
    └── internal/marketdata/binance  (Reader, LoadCandles, FetchAllUSDTSymbols)

internal/app/
    └── internal/render
    └── internal/domain
    └── internal/detect

internal/detect/
    └── internal/domain

internal/render/echarts/
    └── internal/render
    └── internal/domain
    └── internal/detect

internal/artifact/
    └── internal/detect

internal/marketdata/binance/
    └── internal/domain
```

## Layer Overview

| Layer | Package | Responsibility |
|---|---|---|
| Entry point | `cmd/triangled` | CLI wiring, batch + realtime scan loops |
| Domain | `internal/domain` | Shared data types (`Candle`) |
| Detection | `internal/detect` | Ascending triangle pattern detection pipeline |
| Rendering | `internal/render` | `ChartRenderer` interface (port) |
| Rendering (adapter) | `internal/render/echarts` | go-echarts implementation |
| App facade | `internal/app` | `RenderTriangleDetection` orchestration |
| Artifacts | `internal/artifact` | Output file path management and writing |
| Screenshot | `internal/screenshot` | chromedp HTML→PNG conversion |
| Market data | `internal/marketdata/binance` | Binance REST API client |
| Config | `internal/config` | `.env` loading, `AppConfig` |

## Detection Pipeline (`internal/detect`)

1. `stepATR` — calculate ATR and average price
2. `stepVolatilityFilter` — reject if ATR is too low
3. `stepSwingHighs` — find local swing highs
4. `stepFewSwingHighs` — reject if not enough swing highs
5. `stepHorizontalResistance` — find flat resistance level
6. `stepResistanceTouches` — reject if insufficient touches
7. `stepHighBeforeFirstTouch` — validate entry conditions
8. `stepSupportLine` — linear regression on support lows
9. `stepGeometry` — validate triangle shape (slope, r², apex, width)
10. `buildDetectResult` — assemble final `Result`
