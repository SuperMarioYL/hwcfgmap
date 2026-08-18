// Command hwcfgmap probes a 信创 / local GPU box (GPU incl. 国产卡 vendor CLIs,
// CPU, RAM, NVMe) and emits the optimal llama-server launch line for a chosen
// model. It is the probe-and-emit CLI: one command reads the box and writes
// box-tuned llama-server args, collapsing the per-box-per-model trial-and-error
// matrix to a pasteable line.
//
// It never execs llama-server — it only prints the command. The operator
// pastes it into a terminal by hand (the plan names auto-exec explicitly
// out of scope for v0.1).
//
// Usage:
//
//	hwcfgmap probe                       # print BoxProfile JSON
//	hwcfgmap probe --model qwen3-27b     # + synthesised ArgMatrix + launch line
//	hwcfgmap probe --model qwen3-27b --launch-only   # just the pasteable line
//	hwcfgmap models                      # list registered model targets
//	hwcfgmap version
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SuperMarioYL/hwcfgmap/internal/modeltargets"
	"github.com/SuperMarioYL/hwcfgmap/internal/profile"
	"github.com/SuperMarioYL/hwcfgmap/internal/probe"
	"github.com/SuperMarioYL/hwcfgmap/internal/synth"
)

// version is stamped at release time via goreleaser ldflags
// (-ldflags "-X main.version=<v>"). Dev builds keep the -dev suffix.
var version = "0.1.0-dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "hwcfgmap:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return runProbe(nil, stdout, stderr)
	}
	switch args[0] {
	case "probe":
		return runProbe(args[1:], stdout, stderr)
	case "models":
		return runModels(args[1:], stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version)
		return nil
	case "-h", "--help", "help":
		printUsage(stdout)
		return nil
	default:
		// allow `hwcfgmap --model x` as a shorthand for `hwcfgmap probe --model x`
		if strings.HasPrefix(args[0], "-") {
			return runProbe(args, stdout, stderr)
		}
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `hwcfgmap — probe a 信创/local GPU box and emit the optimal llama-server launch line.

usage:
  hwcfgmap probe [--model ID] [--gguf PATH] [--profiles DIR] [--launch-only]
  hwcfgmap models [--profiles DIR]
  hwcfgmap version

probe reads GPU (incl. 国产卡 vendor CLIs: npu-smi/mthreads-smi), CPU, RAM, NVMe
and prints a BoxProfile JSON. With --model it also synthesises the ArgMatrix
and prints a ready-to-paste llama-server line. It never runs llama-server.
`)
}

func runProbe(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		modelID    = fs.String("model", "", "model target id (e.g. qwen3-27b); omit to probe-only")
		ggufPath   = fs.String("gguf", "", "override the --model path baked into the launch line")
		profilesDir = fs.String("profiles", "./profiles", "dir of model-target YAML overrides")
		launchOnly = fs.Bool("launch-only", false, "print only the pasteable llama-server line (for scripts)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Fprintln(stderr, "hwcfgmap: probing box (GPU / CPU / RAM / NVMe)...")
	bp, err := probeBox()
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}

	if *launchOnly {
		if *modelID == "" {
			return fmt.Errorf("--launch-only requires --model")
		}
		mt, line, err := synthesise(bp, *modelID, *ggufPath, *profilesDir)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, line)
		_ = mt
		return nil
	}

	// BoxProfile JSON is the stable m1 contract — always emitted first.
	boxJSON, err := json.MarshalIndent(bp, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, string(boxJSON))

	if *modelID == "" {
		return nil
	}

	mt, line, err := synthesise(bp, *modelID, *ggufPath, *profilesDir)
	if err != nil {
		return err
	}
	amJSON, err := json.MarshalIndent(mt, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "# ArgMatrix")
	fmt.Fprintln(stdout, string(amJSON))
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "# paste into your terminal (edit --model to your GGUF path):")
	fmt.Fprintln(stdout, line)
	return nil
}

// synthesise resolves the model target, runs Synthesize, and renders the line.
// Returns the ArgMatrix (as the synth.Matrix type) + the launch line.
func synthesise(bp profile.BoxProfile, modelID, ggufPath, profilesDir string) (synth.ArgMatrix, string, error) {
	reg := modeltargets.New()
	if err := reg.LoadDir(profilesDir); err != nil {
		return synth.ArgMatrix{}, "", fmt.Errorf("load profiles: %w", err)
	}
	mt, ok := reg.Get(modelID)
	if !ok {
		return synth.ArgMatrix{}, "", fmt.Errorf("unknown model %q (see `hwcfgmap models`)", modelID)
	}
	am := synth.Synthesize(bp, mt)
	line := synth.RenderLaunchLine(am, mt, ggufPath)
	return am, line, nil
}

// runModels lists the registered model targets (in-code defaults + YAML overrides).
func runModels(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("models", flag.ContinueOnError)
	fs.SetOutput(stderr)
	profilesDir := fs.String("profiles", "./profiles", "dir of model-target YAML overrides")
	if err := fs.Parse(args); err != nil {
		return err
	}
	reg := modeltargets.New()
	if err := reg.LoadDir(*profilesDir); err != nil {
		return fmt.Errorf("load profiles: %w", err)
	}
	for _, id := range reg.List() {
		mt, _ := reg.Get(id)
		fmt.Fprintf(stdout, "%-14s  %-22s  q=%s  ctx=%d  layers=%d\n",
			id, mt.Name, mt.DefaultQuant, mt.RecommendedContext, mt.NumLayers)
	}
	return nil
}

// probeBox assembles the full BoxProfile from every probe source. Any single
// source failing (e.g. no GPU CLI on a dev Mac) degrades gracefully rather
// than aborting the whole probe — the box still gets a usable profile.
func probeBox() (profile.BoxProfile, error) {
	bp := profile.BoxProfile{}

	gpus, _ := probe.ProbeGPU()         // empty (not error) when no vendor CLI
	bp.GPU = gpus

	cpu, _ := probe.ProbeCPU()          // always returns a value (degrades to 1 core)
	bp.CPU = cpu

	ram, nvme, _ := probe.ProbeMemory() // nvme best-effort, may be nil
	bp.RAM = ram
	bp.NVMe = nvme

	return bp, nil
}
