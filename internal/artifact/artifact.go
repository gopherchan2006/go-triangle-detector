package artifact

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"triangle-detector/internal/detect"
)

// Names holds all output file paths for one detected pattern.
type Names struct {
	GroupDir   string
	HTMLTmp    string
	PNG        string
	DebugTxt   string
	CalcATRTxt string
	SwingTxt   string
	HorizTxt   string
}

// NewNames builds the full set of artifact paths given a base directory and a stem string.
func NewNames(baseDir, stem string) Names {
	groupDir := filepath.Join(baseDir, stem)
	return Names{
		GroupDir:   groupDir,
		HTMLTmp:    filepath.Join(baseDir, stem+"_render.tmp.html"),
		PNG:        filepath.Join(groupDir, fmt.Sprintf("1_%s_1.png", stem)),
		DebugTxt:   filepath.Join(groupDir, fmt.Sprintf("2_%s_2.txt", stem)),
		CalcATRTxt: filepath.Join(groupDir, fmt.Sprintf("3_%s_calcATR_3.txt", stem)),
		SwingTxt:   filepath.Join(groupDir, fmt.Sprintf("4_%s_findSwingHighs_4.txt", stem)),
		HorizTxt:   filepath.Join(groupDir, fmt.Sprintf("5_%s_findHorizontalResistance_5.txt", stem)),
	}
}

// WriteTexts writes all debug text files for a detected pattern result.
func WriteTexts(names Names, result detect.Result, writeFn func(path string, result detect.Result)) {
	writeFn(names.DebugTxt, result)
	writeLogTxt(names.CalcATRTxt, result.Debug.ATR.CalcATRLog)
	writeLogTxt(names.SwingTxt, result.Debug.Swing.FindSwingHighsLog)
	writeLogTxt(names.HorizTxt, result.Debug.Resistance.FindHorizontalResistanceLog)
}

// WriteLogTxt writes a single log file if content is non-empty.
func WriteLogTxt(path, content string) {
	writeLogTxt(path, content)
}

func writeLogTxt(path, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		log.Printf("writeLogTxt %s: %v", path, err)
	}
}
