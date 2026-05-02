package screenshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

type Screenshotter struct {
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
}

func NewScreenshotter() (*Screenshotter, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.WindowSize(1400, 700),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	if err := chromedp.Run(browserCtx); err != nil {
		browserCancel()
		allocCancel()
		return nil, err
	}

	return &Screenshotter{
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
	}, nil
}

func (s *Screenshotter) Close() {
	s.browserCancel()
	s.allocCancel()
}

func (s *Screenshotter) Screenshot(htmlPath, pngPath string) error {
	absHTML, err := filepath.Abs(htmlPath)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}

	tabCtx, tabCancel := chromedp.NewContext(s.browserCtx)
	defer tabCancel()

	tabCtx, timeoutCancel := context.WithTimeout(tabCtx, 15*time.Second)
	defer timeoutCancel()

	var buf []byte
	fileURL := "file:///" + filepath.ToSlash(absHTML)

	err = chromedp.Run(tabCtx,
		chromedp.EmulateViewport(1400, 700),
		chromedp.Navigate(fileURL),
		chromedp.WaitReady("canvas", chromedp.ByQuery),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.FullScreenshot(&buf, 90),
	)
	if err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}

	return os.WriteFile(pngPath, buf, 0o644)
}
