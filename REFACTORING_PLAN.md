# лан рефакторинга go-triangle-detector

равило каждого коммита: `go build ./...` зелёный, одно логическое изменение, PR-diff читается за 5 минут.

---

## 1. удит текущего состояния

### 1.1 роблемы архитектуры

| # | айл / место | роблема | атегория |
|---|---|---|---|
| 1 | есь проект | сё в `package main` — нет изоляции домена от CLI/HTTP/FS | рхитектура |
| 2 | `detector.go` | ~500 строк: типы + debug-снимки + форматирование + pipeline + math | God Object |
| 3 | `detector.go` | `reject(reason, map[string]*int)` — доменная функция мутирует инфраструктурный счётчик | арушение SRP / DIP |
| 4 | `detector.go` | 25+ строковых литералов причин отказа — нет типобезопасности, опечатка == тихий баг | агические строки |
| 5 | `detector.go` | агические числа (`0.005`, `0.7`, `15`, `40`, `0.85`…) разбросаны по коду | агические числа |
| 6 | `main.go` | `writeDebugTxt`, `writeCalcATRDebugTxt` и ещё 3 — почти одинаковые функции записи | ублирование |
| 7 | `main.go` + `realtime.go` | огика «сохранить бандл артефактов» дублируется в двух местах | DRY |
| 8 | `loader.go` | рямые `http.Get` без `context.Context` — нельзя отменить по Ctrl+C | адёжность |
| 9 | `loader.go` | `defer resp.Body.Close()` в цикле пагинации — утечка handle до конца функции | есурсы |
| 10 | `loader.go` | Retry захардкожен, нет интерфейса — нельзя подставить мок | Тестируемость |
| 11 | `realtime.go` | оркеры без ctx.Done() — Ctrl+C не останавливает горутины | Graceful shutdown |
| 12 | `main.go` | `os.RemoveAll("tmp")` без подтверждения в начале main | езопасность данных |
| 13 | есь проект | ет ни одного `*_test.go` | Тестируемость |
| 14 | `config.go` | env-переменные читаются точечно в `main.go` | онфигурация |
| 15 | `DebugInfo` struct | 30+ плоских полей разных фаз перемешаны в одну структуру | Связность |

### 1.2 то уже хорошо

| то | очему хорошо |
|---|---|
| `ChartRenderer` interface | равильный порт — легко добавить другой рендерер |
| `echarts_renderer.go` отделён | даптер изолирован от пайплайна |
| Worker pool в `scanAllSymbols` | орректный паттерн конкурентности |
| `detectAscendingTriangle` — чистая функция без глобального состояния | Unit-тестируемая |
| остраничная загрузка в `LoadCandles` | равильная работа с Binance API |

---

## 2. елевая архитектура

### 2.1 Структура пакетов

```
cmd/
  triangled/
    main.go             # точка входа: флаги, wiring зависимостей

internal/
  domain/
    candle.go           # type Candle struct (Value Object)

  detect/
    params.go           # DetectorParams + DefaultDetectorParams()
    reason.go           # type RejectReason string + const блок
    result.go           # AscendingTriangleResult, DebugInfo (вложенные sub-struct)
    detector.go         # пайплайн + публичный API
    atr.go              # calcATR, collect*, format*
    swing.go            # findSwingHighs, collect*, format*
    resistance.go       # findHorizontalResistance, collect*, format*
    valleys.go          # findValleysBetweenTouches
    math.go             # linearRegression, rSquared
    trace.go            # DetectOption, WithTrace, WithParams
    counter.go          # RejectCounter interface + MapCounter + NoopCounter
    detect_test.go      # unit-тесты на синтетических candles

  marketdata/
    port.go             # KlineReader interface, KlineQuery
    binance/
      client.go         # BinanceClient{httpClient, baseURL}
      parser.go         # parseKlines
      client_test.go    # httptest.Server mock

  artifact/
    bundle.go           # ExportPatternBundle(...)
    namer.go            # ArtifactNames struct
    writer.go           # writeText, writeDebug

  render/
    port.go             # ChartRenderer interface
    echarts/
      renderer.go       # EChartsRenderer
    render.go           # RenderTriangleDetection

  screenshot/
    screenshotter.go    # Screenshotter interface + chromedp impl

  app/
    batch.go            # BatchAnalyzer.AnalyzeSymbol(ctx, cfg) — Facade
    realtime.go         # RealtimeAnalyzer.Run(ctx, cfg) — Facade

  config/
    env.go              # LoadAppConfig() AppConfig
    app_config.go       # type AppConfig struct
```

### 2.2 иаграмма зависимостей

```
cmd/triangled
    ↓ wires
internal/app
    ↓ uses
internal/detect      (чистая логика, zero I/O, zero OS)
internal/marketdata  (порт + Binance адаптер)
internal/artifact    (запись файлов)
internal/render      (ChartRenderer)
internal/screenshot  (chromedp)
internal/config      (env)
```

`internal/detect` не импортирует ничего кроме `internal/domain`. ависимости в одну сторону — внутрь.

---

## 3. аттерны и их применение

### 3.1 Hexagonal Architecture (Ports and Adapters)

**орты — интерфейсы на границе домена и инфраструктуры:**

```go
// internal/marketdata/port.go
type KlineReader interface {
    FetchKlines(ctx context.Context, q KlineQuery) ([]domain.Candle, error)
    FetchLastN(ctx context.Context, symbol, interval string, n int) ([]domain.Candle, error)
}

// internal/screenshot/screenshotter.go
type Screenshotter interface {
    Screenshot(htmlPath, pngPath string) error
    Close() error
}
```

**даптер — реализует порт через конкретную технологию:**

```go
// internal/marketdata/binance/client.go
type BinanceClient struct {
    http    *http.Client
    baseURL string
}

func (c *BinanceClient) FetchKlines(ctx context.Context, q KlineQuery) ([]domain.Candle, error) { … }
```

Тест подставляет `StaticReader{candles: fixture}` — никакой сети.

---

### 3.2 Functional Options (для детектора)

```go
// internal/detect/trace.go
type DetectOption func(*detectOpts)

type detectOpts struct {
    params  DetectorParams
    trace   bool
    counter RejectCounter
}

func WithTrace(on bool) DetectOption           { return func(o *detectOpts) { o.trace = on } }
func WithParams(p DetectorParams) DetectOption { return func(o *detectOpts) { o.params = p } }
func WithCounter(c RejectCounter) DetectOption { return func(o *detectOpts) { o.counter = c } }

func DetectAscendingTriangle(candles []domain.Candle, opts ...DetectOption) AscendingTriangleResult {
    o := detectOpts{params: DefaultDetectorParams(), trace: true, counter: NoopCounter{}}
    for _, opt := range opts {
        opt(&o)
    }
    return detectAscendingTriangle(candles, o)
}
```

`DetectAscendingTriangle(window)` — обратная совместимость без изменений.
 hot-path: `DetectAscendingTriangle(window, WithTrace(false))` — пропускает все `format*` вызовы.

---

### 3.3 Value Object — RejectReason

```go
// internal/detect/reason.go
type RejectReason string

const (
    ReasonFewSwingHighs        RejectReason = "01_few_swing_highs"
    ReasonResistanceLt3Touches RejectReason = "02_resistance_lt3_touches"
    ReasonHighBeforeFirstTouch RejectReason = "03_high_before_first_touch"
    ReasonCrashBeforeFirstTouch RejectReason = "04_crash_before_first_touch"
    ReasonFirstTouchTooLate    RejectReason = "05_first_touch_too_late"
    ReasonFewValleys           RejectReason = "06_few_valleys"
    ReasonValleyNotRising      RejectReason = "07_valley_not_rising"
    ReasonNegativeSlope        RejectReason = "08_negative_slope"
    ReasonValleyTooDeep        RejectReason = "09_valley_too_deep"
    ReasonLowRSquared          RejectReason = "10_low_r_squared"
    // … остальные 15
)
```

Строковые значения заморожены — совпадают с именами папок в `tmp/rejects/`. IDE-rename не ломает диск.

---

### 3.4 Observer / Strategy — RejectCounter

```go
// internal/detect/counter.go
type RejectCounter interface {
    Inc(reason RejectReason)
}

// ля production CLI
type MapCounter struct {
    mu sync.Mutex
    m  map[RejectReason]int
}
func (c *MapCounter) Inc(r RejectReason) { c.mu.Lock(); c.m[r]++; c.mu.Unlock() }
func (c *MapCounter) Snapshot() map[RejectReason]int { … }

// ля realtime (статистика не нужна) и hot-path
type NoopCounter struct{}
func (NoopCounter) Inc(RejectReason) {}

// ля тестов
type SliceCounter struct{ Reasons []RejectReason }
func (c *SliceCounter) Inc(r RejectReason) { c.Reasons = append(c.Reasons, r) }
```

`MapCounter` потокобезопасен — параллельный realtime-скан пишет в один счётчик без гонок.

---

### 3.5 Pipeline / Chain of Steps

```go
// internal/detect/detector.go
type pipeStep func(*pipeCtx) (rejected bool)

func detectAscendingTriangle(candles []domain.Candle, o detectOpts) AscendingTriangleResult {
    ctx := newPipeCtx(candles, o)

    steps := []pipeStep{
        stepCalcATR,
        stepFindSwingHighs,
        stepFindResistance,
        stepCheckTimingAndHighs,   // фильтры 03, 04, 05
        stepFindValleys,
        stepValidateValleys,       // фильтры 06-11
        stepFitSupportLine,
        stepCheckGeometry,         // фильтры 12-19
        stepCheckPrecedingTrend,
        stepCheckVolume,
    }

    for _, step := range steps {
        if rejected := step(ctx); rejected {
            return ctx.result()
        }
    }
    ctx.markFound()
    return ctx.result()
}
```

овая проверка = одна функция + одна строка в слайсе.

---

### 3.6 Facade — internal/app

```go
// internal/app/batch.go
type BatchConfig struct {
    Symbol      string
    Interval    string
    StartDate   string
    EndDate     string
    DataDir     string
    RejectLimit int
    WindowSize  int
}

type BatchAnalyzer struct {
    reader       marketdata.KlineReader
    renderer     render.Factory
    screenshots  screenshot.Screenshotter
    artifacts    artifact.Bundler
    params       detect.DetectorParams
}

func (a *BatchAnalyzer) AnalyzeSymbol(ctx context.Context, cfg BatchConfig) error {
    candles, err := a.reader.FetchKlines(ctx, toQuery(cfg))
    // … sliding window → detect → export bundle
}
```

`cmd/triangled/main.go` создаёт `BatchAnalyzer` с реальными зависимостями. Тест — с моками.

---

### 3.7 Builder — вложенный DebugInfo

о (30+ плоских полей):
```go
type DebugInfo struct {
    AvgPrice float64; ATR float64; Vol float64; CalcATRLog string
    SwingHighsCount int; FindSwingHighsLog string
    ResistanceLevel float64; ResistanceTouches int; // …
}
```

осле (вложенные sub-struct):
```go
type DebugInfo struct {
    ATR        ATRDebug
    Swing      SwingDebug
    Resistance ResistanceDebug
    Support    SupportDebug
    Geometry   GeometryDebug
}

type ATRDebug struct {
    AvgPrice float64
    ATR      float64
    Vol      float64
    Log      string  // только при trace=true
}

type GeometryDebug struct {
    PatternStart  int
    PatternEnd    int
    HeightAtStart float64
    HeightAtEnd   float64
    XIntersect    float64
    PatternWidth  float64
}
```

---

### 3.8 Worker Pool (формализация существующего)

```go
// internal/app/realtime.go
func scanAllSymbols(ctx context.Context, symbols []string, cfg RealtimeConfig, reader marketdata.KlineReader) []scanResult {
    jobs    := make(chan string, len(symbols))
    results := make(chan scanResult, len(symbols))

    var wg sync.WaitGroup
    for i := 0; i < cfg.Workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for sym := range jobs {
                select {
                case <-ctx.Done():
                    return
                default:
                }
                // fetch → detect → send to results
            }
        }()
    }

    for _, s := range symbols { jobs <- s }
    close(jobs)
    wg.Wait()
    close(results)
    // …
}
```

`ctx.Done()` — `Ctrl+C` останавливает все воркеры gracefully.

---

### 3.9 Context Propagation

```go
// cmd/triangled/main.go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

// внутри каждого HTTP-запроса
reqCtx, reqCancel := context.WithTimeout(ctx, 10*time.Second)
defer reqCancel()
candles, err := reader.FetchKlines(reqCtx, query)
```

---

### 3.10 Typed Errors

```go
// internal/marketdata/errors.go
type APIError struct {
    StatusCode int
    Body       string
}
func (e *APIError) Error() string { return fmt.Sprintf("binance API %d: %s", e.StatusCode, e.Body) }

// caller
var apiErr *APIError
if errors.As(err, &apiErr) && apiErr.StatusCode == 429 {
    // exponential backoff
}
```

---

### 3.11 Artifact Bundle — устранение дублирования

```go
// internal/artifact/bundle.go
type BundleInput struct {
    Dir           string
    Stem          string
    Window        []domain.Candle
    Result        detect.AscendingTriangleResult
    Renderer      render.ChartRenderer
    Screenshotter screenshot.Screenshotter // nil → пропустить PNG
}

func ExportPatternBundle(ctx context.Context, in BundleInput) error {
    names := ArtifactNames(in.Dir, in.Stem)
    // render HTML → screenshot PNG → writeDebug → cleanup HTML
}
```

---

### 3.12 Anti-patterns — что  делать

| Anti-pattern | очему не нужен |
|---|---|
| God interface на 15+ методов | ет второй реализации — интерфейс преждевременен |
| `util/` / `common/` пакет | Создаёт циклические зависимости |
| Event bus / message broker | збыточно для single-binary |
| DDD Aggregate + Event Sourcing | ет персистентности состояния паттернов |
| енерация кода (protobuf, wire) | роект слишком маленький |

---

## 4. HTTP и надёжность

- дин `*http.Client{Timeout: 15s}` — передаётся в `BinanceClient`, не создаётся на каждый запрос.
- `FetchKlines` и `FetchLastN` принимают `context.Context`.
-  цикле пагинации: `io.ReadAll` + явный `resp.Body.Close()` вместо `defer` внутри цикла.
- Retry с backoff перенести в единый метод `BinanceClient.do(req)`.
- `isNetworkError` → переехать в `internal/marketdata/binance/`.

---

## 5. Тестирование

| ровень | айл | то проверяем |
|---|---|---|
| Unit | `internal/detect/detect_test.go` | Found/Reject на synthetic fixtures |
| Unit | `internal/detect/math_test.go` | `linearRegression`, `rSquared` |
| Unit | `internal/detect/atr_test.go` | `calcATR` против ручного расчёта |
| Unit | `internal/artifact/namer_test.go` | равильность путей файлов |
| Integration | `internal/marketdata/binance/client_test.go` | `httptest.Server` + JSON-фикстура |

ервый тест: `TestDetectFound` — синтетические 50 свечей с идеальным треугольником, `Found == true`.

---

## 6. азбивка на коммиты

осле каждого коммита: `go build ./...` должен быть зелёным.

---

### аза A — типобезопасность (нулевой риск для логики)

#### A-01 `feat: add RejectReason type and const block`
- Создать `reject_reason.go` в `package main`
- `type RejectReason string`
- `const` блок с 25 причинами — строки те же что в коде

#### A-02 `refactor: replace reject() string literals with RejectReason constants`
- зменить сигнатуру `reject(reason RejectReason, …)`
- аменить все `"01_few_swing_highs"` → `ReasonFewSwingHighs` и т.д.
- Только механическая замена

#### A-03 `refactor: move AscendingTriangleResult and DebugInfo to detector_types.go`
- овый файл, тот же `package main`
- Cut+Paste: `AscendingTriangleResult`, `DebugInfo`, `SwingPoint`

#### A-04 `refactor: move debug snapshot types to detector_types.go`
- ополнить: `CalcATRBarTrace`, `CalcATRDebugSnapshot`, `SwingHighScanRow`, `FindSwingHighsDebugSnapshot`, `HorizontalResistanceGroupDebug`, `FindHorizontalResistanceDebugSnapshot`
- тдельный коммит — diff читаем

---

### аза B — параметры детектора

#### B-01 `feat: add DetectorParams struct with DefaultDetectorParams()`
- Создать `params.go`
- се магические числа как поля с говорящими именами
- `Default()` возвращает те же значения что в коде сейчас

#### B-02 `refactor: use DetectorParams.SwingRadius`
- робросить `params DetectorParams` в `detectAscendingTriangle`
- аменить `radius := 3` → `p.SwingRadius`

#### B-03 `refactor: use DetectorParams for ATR/vol tolerances`
- `0.005` → `p.VolTolerance`; `0.015` → `p.BreakoutTolerance`

#### B-04 `refactor: use DetectorParams for timing thresholds`
- `0.4` → `p.FirstTouchMaxRatio`; `5` → `p.MinResistanceSpacing`

#### B-05 `refactor: use DetectorParams for valley/slope thresholds`
- `0.85` → `p.MinRSquared`; `0.7` → `p.MaxNarrowingRatio`

#### B-06 `refactor: use DetectorParams for geometry thresholds`
- `15` → `p.MinPatternWidth`; `2.0` → `p.MaxApexFactor`

#### B-07 `refactor: use DetectorParams for valley depth thresholds`
- `0.05` / `0.08` / `0.15` → соответствующие поля

#### B-08 `refactor: DetectAscendingTriangle accepts DetectorParams`
- бновить публичный API + все вызовы в `main.go`, `realtime.go`

---

### аза C — Functional Options

#### C-01 `feat: add DetectOption type and WithTrace/WithParams functions`
- овый `detect_opts.go`: `type DetectOption func(*detectOpts)`, `WithTrace`, `WithParams`

#### C-02 `refactor: DetectAscendingTriangle uses variadic DetectOption`
- `DetectAscendingTriangle(candles []Candle, opts ...DetectOption)`
- брать явный `DetectorParams` параметр
- ызывающие без изменений

#### C-03 `feat: WithTrace(false) skips all format* calls`
-  `collectCalcATRDebug` и др. — если `!opts.trace` не вызывать `format*`
- ервый test файл: `detect_test.go` — `Log == ""` при trace=false

#### C-04 `test: calcATR unit test with manual expected value`

---

### аза D — RejectCounter интерфейс

#### D-01 `feat: add RejectCounter interface, NoopCounter, MapCounter`
- обавить в `reject_counter.go`

#### D-02 `refactor: reject() accepts RejectCounter`
- `reject(reason RejectReason, result *AscendingTriangleResult, c RejectCounter)`
- `c.Inc(reason)` вместо map-мутации

#### D-03 `feat: add WithCounter DetectOption`

#### D-04 `refactor: main.go uses MapCounter`
- `counter := &MapCounter{m: make(map[RejectReason]int)}`
- `WithCounter(counter)` в вызовах

#### D-05 `refactor: realtime.go uses NoopCounter`

#### D-06 `test: SliceCounter captures reject reasons`
- `TestRejectFewSwingHighs` — на слишком коротком срезе

---

### аза E — разрезание detector.go

#### E-01 `refactor: extract ATR funcs to detector_atr.go`
- `calcATR`, `collectCalcATRDebug`, `formatCalcATRDebug`

#### E-02 `refactor: extract swing funcs to detector_swing.go`
- `findSwingHighs`, `collectFindSwingHighsDebug`, `formatFindSwingHighsDebug`

#### E-03 `refactor: extract resistance funcs to detector_resistance.go`
- `findHorizontalResistance`, `collectFindHorizontalResistanceDebug`, `formatFindHorizontalResistanceDebug`

#### E-04 `refactor: extract math funcs to detector_math.go`
- `linearRegression`, `rSquared`, `findValleysBetweenTouches`

#### E-05 `refactor: detector.go contains only pipeline + public API`
- бедиться файл ≤150 строк

---

### аза F — артефакты

#### F-01 `feat: add ArtifactNames helper`
- `type ArtifactNames struct{ HTML, PNG, DebugTxt, ATRTxt, SwingTxt, HorizTxt string }`
- `func NewArtifactNames(dir, stem string) ArtifactNames`

#### F-02 `refactor: main.go uses ArtifactNames`
- аменить 5 отдельных `filepath.Join` вызовов на `NewArtifactNames`

#### F-03 `refactor: merge 5 write* functions into writeArtifactTexts`
- `func writeArtifactTexts(names ArtifactNames, result AscendingTriangleResult)`

#### F-04 `feat: add ExportPatternBundle`
- дин вызов вместо render+screenshot+write

#### F-05 `refactor: main.go analyzeSymbol uses ExportPatternBundle`

#### F-06 `refactor: realtime.go runCycle uses ExportPatternBundle`

---

### аза G — Pipeline Steps

#### G-01 `refactor: introduce pipeCtx and extract stepCalcATR`
- `type pipeCtx struct`; `type pipeStep func(*pipeCtx) bool`
- ынести первые ~30 строк пайплайна

#### G-02 `refactor: extract stepFindSwingHighs`
#### G-03 `refactor: extract stepFindResistance`
#### G-04 `refactor: extract stepCheckTimingAndHighs` (фильтры 03-05)
#### G-05 `refactor: extract stepFindValleys`
#### G-06 `refactor: extract stepValidateValleys` (фильтры 06-11)
#### G-07 `refactor: extract stepFitSupportLine`
#### G-08 `refactor: extract stepCheckGeometry` (фильтры 12-19)
#### G-09 `refactor: extract stepCheckPrecedingTrend`
#### G-10 `refactor: extract stepCheckVolume`

#### G-11 `refactor: pipeline loop replaces if-chain`
- `for _, step := range steps { if step(ctx) { return ctx.result() } }`

---

### аза H — DebugInfo вложенные struct

#### H-01 `refactor: nest ATR fields → DebugInfo.ATR`
#### H-02 `refactor: nest swing fields → DebugInfo.Swing`
#### H-03 `refactor: nest resistance fields → DebugInfo.Resistance`
#### H-04 `refactor: nest support line fields → DebugInfo.Support`
#### H-05 `refactor: nest geometry fields → DebugInfo.Geometry`
#### H-06 `refactor: update writeDebugTxt for nested struct`

---

### аза I — HTTP / надёжность

#### I-01 `fix: close response body immediately after ReadAll in pagination loop`
- брать `defer` внутри цикла; явный `resp.Body.Close()` сразу

#### I-02 `feat: add KlineReader interface`

#### I-03 `feat: add BinanceReader implementing KlineReader`

#### I-04 `refactor: add context.Context to LoadCandles`

#### I-05 `refactor: add context.Context to LoadLastNCandles`

#### I-06 `refactor: add context.Context to FetchAllUSDTSymbols`

#### I-07 `feat: BinanceReader uses http.Client with 15s timeout`

#### I-08 `feat: add typed APIError`

---

### аза J — Graceful Shutdown

#### J-01 `feat: signal.NotifyContext in main`
- `ctx, cancel := signal.NotifyContext(…, os.Interrupt, syscall.SIGTERM)`

#### J-02 `refactor: scanAllSymbols respects ctx.Done()`

#### J-03 `refactor: FetchLastN retry loop checks ctx.Done()`

---

### аза K — онфигурация

#### K-01 `feat: add AppConfig consolidating env reads`
#### K-02 `refactor: main.go uses AppConfig`

---

### аза L — еренос в cmd/ и internal/

#### L-01 `refactor: move Candle to internal/domain/candle.go`
#### L-02 `refactor: move detect package to internal/detect/`
#### L-03 `refactor: move render port to internal/render/`
#### L-04 `refactor: move artifact helpers to internal/artifact/`
#### L-05 `refactor: move screenshot to internal/screenshot/`
#### L-06 `refactor: move marketdata to internal/marketdata/`
#### L-07 `refactor: move config to internal/config/`
#### L-08 `feat: add internal/app/batch.go Facade`
#### L-09 `feat: add internal/app/realtime.go Facade`
#### L-10 `feat: add cmd/triangled/main.go; remove root main.go`

---

### аза M — Тесты

#### M-01 `test: TestDetectFound on synthetic ideal triangle`
#### M-02 `test: TestReject* for most common reject reasons`
#### M-03 `test: linearRegression and rSquared`
#### M-04 `test: calcATR matches manual calculation`
#### M-05 `test: BinanceClient with httptest.Server`
#### M-06 `test: ArtifactNames generates expected paths`

---

### аза N — инальная полировка

#### N-01 `docs: add ARCHITECTURE.md with package dependency graph`
#### N-02 `ci: add Makefile with build/test/lint targets`
#### N-03 `ci: add .golangci.yml (errcheck, govet, staticcheck, unused)`
#### N-04 `chore: add tmp/ to .gitignore`
#### N-05 `docs: update README with new structure and usage`

---

## 7. Таблица приоритетов

| аза | иск | ыгода | елать первым? |
|---|---|---|---|
| A — константы/типы | нулевой | читаемость, IDE-navigate | а |
| B — параметры | минимальный | конфигурируемость, документируемые пороги | а |
| C — options | низкий | trace off в hot-path | а |
| D — counter | низкий | тестируемость reject | а |
| E — разрезание файла | низкий | навигация | осле B |
| F — артефакты | средний | устранение DRY | осле D |
| G — pipeline steps | средний | читаемость пайплайна | осле E |
| H — DebugInfo nested | средний | структура данных | осле G |
| I — HTTP/context | высокий | надёжность, real bug fix | I-01 сразу |
| J — graceful shutdown | высокий | корректное завершение | осле I |
| L — internal/ перенос | высокий | чистая архитектура | оследним |
| M — тесты | низкий | уверенность | араллельно |

---

## 8. тог

**инимально полезный набор (если мало времени):**

1. **I-01** — fix body leak: 1 строка, 5 минут, реальный bug.
2. **аза A** — 4 коммита, ~2 часа, нулевой риск, магические строки уходят.
3. **аза B** — 8 коммитов, ~1 день, все пороги в одном месте с именами.
4. **аза E** — 5 коммитов, ~полдня, `detector.go` из 500 строк в 5 читаемых файлов.
5. **J-01** — graceful shutdown: `Ctrl+C` перестаёт убивать процесс без сохранения.

осле этого пяти пунктов: детектор тестируем, пороги документированы, файловая структура понятна, CI-ready.
