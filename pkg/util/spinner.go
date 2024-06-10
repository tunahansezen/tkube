package util

import (
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/guumaster/logsymbols"
	"time"
)

var (
	spin *spinner.Spinner
)

func GetSpinner() *spinner.Spinner {
	return spin
}

func StartSpinner(suffix string) {
	spin = spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	spin.Suffix = fmt.Sprintf(" %s", suffix)
	spin.Start()
}

func UpdateSpinner(suffix string) {
	spin.Suffix = fmt.Sprintf(" %s", suffix)
}

func StopSpinner(finalMsg string, symbol logsymbols.Symbol) {
	if spin == nil {
		return
	}
	if finalMsg != "" {
		spin.FinalMSG = fmt.Sprintf("%s %s\n", symbol, finalMsg)
	} else {
		spin.FinalMSG = fmt.Sprintf("%s%s\n", symbol, spin.Suffix)
	}
	spin.Stop()
	spin = nil
}
