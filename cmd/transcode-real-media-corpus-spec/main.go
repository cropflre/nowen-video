package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

type report struct {
	SchemaVersion string               `json:"schema_version"`
	SpecVersion   string               `json:"spec_version"`
	SpecHash      string               `json:"spec_hash"`
	Spec          transcodecorpus.Spec `json:"spec"`
}

func main() {
	output := flag.String("output", "", "write the canonical corpus spec report to this path")
	list := flag.Bool("list", false, "list registered corpus case IDs")
	flag.Parse()

	spec := transcodecorpus.DefaultSpec()
	version, hash, _, err := transcodecorpus.SpecIdentity(spec)
	if err != nil {
		fatal(err)
	}
	if *list {
		for _, caseSpec := range spec.Cases {
			fmt.Printf("%s\t%s\n", caseSpec.ID, caseSpec.Description)
		}
		return
	}

	payload, err := json.MarshalIndent(report{
		SchemaVersion: "real-media-corpus-spec-report-v1",
		SpecVersion:   version,
		SpecHash:      hash,
		Spec:          spec,
	}, "", "  ")
	if err != nil {
		fatal(err)
	}
	payload = append(payload, '\n')
	if *output == "" {
		if _, err := os.Stdout.Write(payload); err != nil {
			fatal(err)
		}
		return
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*output, payload, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s\n", *output)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
