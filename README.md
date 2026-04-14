
# Triangle Detector

Simple Go project for detecting triangle patterns in candlestick data and rendering charts.

## Usage

- Read local `candles.json` (default when file exists):
  ```sh
  go run .
  ```

- Explicitly fetch from Binance (won't overwrite an existing non-empty `candles.json`):
  ```sh
  go run . -symbol BTCUSDT -interval 15m -start 2026-04-14T00:00:00Z -end 2026-04-14T13:00:00Z
  ```

- If `candles.json` is missing or empty, running without flags will fetch defaults (BTCUSDT @ 15m) and save the file.

## Next steps

- Add `-force` flag to allow intentional overwrite of `candles.json`.
- Add `-count`/`-last` flags to request a specific number of candles.
- Improve CLI messaging and add tests for `LoadCandles` and helpers.

## Contact

- Telegram: @gof4rvr
- Email: r3ndyhell@gmail.com

