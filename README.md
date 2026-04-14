

# Triangle Detector

Usage

- Read local `candles.json` (default when file exists):
  ```sh
  go run .
  ```

- Explicitly fetch from Binance (won't overwrite `candles.json` unless `-force` is used):
  ```sh
  go run . -symbol BTCUSDT -interval 15m -start 2026-04-14T00:00:00Z -end 2026-04-14T13:00:00Z
  ```

- Overwrite behavior:
  - By default the loader will NOT overwrite an existing non-empty `candles.json`.
  - To force overwrite when fetching, pass `-force`.

Contact

- Telegram: @gof4rvr
- Email: r3ndyhell@gmail.com

