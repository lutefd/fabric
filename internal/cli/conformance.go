package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lutefd/fabric/protocol"
)

type conformanceReport struct {
	Checked int      `json:"checked"`
	Valid   int      `json:"valid"`
	Invalid []string `json:"invalid,omitempty"`
}

func runConformance(args []string) error {
	fs := flag.NewFlagSet("conformance", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	files := stringListFlag{}
	fs.Var(&files, "file", "event envelope file, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	files = append(files, fs.Args()...)
	if len(files) == 0 {
		entries, err := os.ReadDir(ledgerEventsPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				files = append(files, filepath.Join(ledgerEventsPath, entry.Name()))
			}
		}
	}
	if len(files) == 0 {
		return errors.New("conformance requires event files or an initialized ledger")
	}
	report := conformanceReport{}
	for _, path := range files {
		report.Checked++
		raw, err := os.ReadFile(path)
		if err != nil {
			report.Invalid = append(report.Invalid, path+": "+err.Error())
			continue
		}
		if _, err := protocol.DecodeEvent(raw); err != nil {
			report.Invalid = append(report.Invalid, path+": "+err.Error())
			continue
		}
		report.Valid++
	}
	setMachineResult(report)
	fmt.Printf("Conformance: %d valid of %d checked.\n", report.Valid, report.Checked)
	for _, invalid := range report.Invalid {
		fmt.Printf("- %s\n", invalid)
	}
	if len(report.Invalid) > 0 {
		return typedError("conformance_failed", fmt.Sprintf("%d conformance failures", len(report.Invalid)), map[string]any{
			"checked": report.Checked,
			"valid":   report.Valid,
			"invalid": report.Invalid,
		})
	}
	return nil
}
