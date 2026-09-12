package synth

import (
	"fmt"
	"strings"

	"github.com/SuperMarioYL/hwcfgmap/internal/modeltargets"
)

// RenderLaunchLine turns an ArgMatrix (+ the model it was synthesised for, and
// an optional operator-supplied GGUF path) into the pasteable llama-server
// command line. It is the "emit" half of probe-and-emit — it builds a string,
// it never execs anything (the operator pastes it into a terminal by hand).
//
// Flag order is the canonical llama-server layout: --model, --n-gpu-layers,
// -c, -b, -t, --cache-type-k, then --mlock when the matrix opts in.
func RenderLaunchLine(am ArgMatrix, mt modeltargets.ModelTarget, ggufPath string) string {
	path := ggufPath
	if path == "" {
		// conventional name the operator is likely to have on disk; editable.
		path = "./" + mt.ID + "-" + am.Quant + ".gguf"
	}
	exec := am.ExecBinary
	if exec == "" {
		exec = "llama-server"
	}
	parts := []string{
		exec,
		"--model", shellQuote(path),
		"--n-gpu-layers", fmt.Sprintf("%d", am.NGPULayers),
		"-c", fmt.Sprintf("%d", am.ContextSize),
		"-b", fmt.Sprintf("%d", am.BatchSize),
		"-t", fmt.Sprintf("%d", am.Threads),
		"--cache-type-k", am.KVCacheType,
	}
	if am.Mlock {
		parts = append(parts, "--mlock")
	}
	return strings.Join(parts, " ")
}

// shellQuote returns s unchanged when it holds only characters a POSIX shell
// passes through inside a bare word — keeping every historical rendering of
// the launch line byte-identical — and otherwise wraps it in single quotes
// (escaping embedded single quotes with the standard close-quote/backslash/
// reopen-quote idiom), so a pasted line always hands the server exactly one
// --model argument even for paths with spaces or other shell metacharacters.
func shellQuote(s string) string {
	if s == "" {
		return s
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '/' || c == '.' || c == '_' || c == '-' || c == '=' || c == ':' || c == '~':
		default:
			return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
		}
	}
	return s
}
