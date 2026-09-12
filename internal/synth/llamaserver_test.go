package synth

import (
	"os/exec"
	"testing"

	"github.com/SuperMarioYL/hwcfgmap/internal/modeltargets"
)

func testMatrix() ArgMatrix {
	return ArgMatrix{
		NGPULayers: 58, ContextSize: 8192, BatchSize: 512, Threads: 8,
		KVCacheType: "q8_0", Mlock: false, Quant: "q4_k_m", ExecBinary: "llama-server",
	}
}

// A --gguf path with whitespace must render quoted, so a pasted line hands
// llama-server exactly one --model argument (the initial release emitted
// the raw path and the shell split it).
func TestRenderLaunchLine_QuotesSpacedPath(t *testing.T) {
	line := RenderLaunchLine(testMatrix(), modeltargets.Qwen3_27B, "/data/models/my model.gguf")
	want := "llama-server --model '/data/models/my model.gguf' --n-gpu-layers 58 -c 8192 -b 512 -t 8 --cache-type-k q8_0"
	if line != want {
		t.Fatalf("spaced path mismatch:\nwant %q\ngot  %q", want, line)
	}
}

func TestRenderLaunchLine_QuotesSingleQuotePath(t *testing.T) {
	line := RenderLaunchLine(testMatrix(), modeltargets.Qwen3_27B, "/data/mod'els/q.gguf")
	want := "llama-server --model '/data/mod'\\''els/q.gguf' --n-gpu-layers 58 -c 8192 -b 512 -t 8 --cache-type-k q8_0"
	if line != want {
		t.Fatalf("quoted path mismatch:\nwant %q\ngot  %q", want, line)
	}
}

// Clean paths render byte-identical to the initial release — the quoting only activates
// for characters a shell would split or reinterpret.
func TestRenderLaunchLine_CleanPathsUnchanged(t *testing.T) {
	def := RenderLaunchLine(testMatrix(), modeltargets.Qwen3_27B, "")
	wantDef := "llama-server --model ./qwen3-27b-q4_k_m.gguf --n-gpu-layers 58 -c 8192 -b 512 -t 8 --cache-type-k q8_0"
	if def != wantDef {
		t.Fatalf("default path rendering changed:\nwant %q\ngot  %q", wantDef, def)
	}
	clean := RenderLaunchLine(testMatrix(), modeltargets.Qwen3_27B, "/data/models/q.gguf")
	wantClean := "llama-server --model /data/models/q.gguf --n-gpu-layers 58 -c 8192 -b 512 -t 8 --cache-type-k q8_0"
	if clean != wantClean {
		t.Fatalf("clean path rendering changed:\nwant %q\ngot  %q", wantClean, clean)
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"clean":                "clean",
		"/data/x-1_2.v3=y:z~w": "/data/x-1_2.v3=y:z~w",
		"":                     "",
		"/data/my model.gguf":  "'/data/my model.gguf'",
		"it's here.gguf":       "'it'\\''s here.gguf'",
		"*.gguf":               "'*.gguf'",
		"$HOME/model.gguf":     "'$HOME/model.gguf'",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Fatalf("shellQuote(%q): want %q, got %q", in, want, got)
		}
	}
}

// End-to-end: a POSIX shell must parse the emitted launch line so that the
// --model value is exactly one positional argument equal to the original
// path. `set -- <line>` loads the words without executing anything; $3 is
// the --model value ($1=exec, $2=--model).
func TestRenderLaunchLine_ShellParsesOneModelArg(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on PATH — cannot verify shell parsing")
	}
	for _, path := range []string{
		"/data/models/my model.gguf",
		"/data/mod'els/q.gguf",
		"/data/models/q.gguf",
	} {
		line := RenderLaunchLine(testMatrix(), modeltargets.Qwen3_27B, path)
		script := "set -- " + line + "\nprintf '%s' \"$3\"\n"
		out, err := exec.Command(sh, "-c", script).Output()
		if err != nil {
			t.Fatalf("sh parse of %q failed: %v", line, err)
		}
		if string(out) != path {
			t.Fatalf("shell parsed --model as %q, want the single argument %q", string(out), path)
		}
	}
}
