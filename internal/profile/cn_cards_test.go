package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// npuSMI910B1 is a verbatim captured `npu-smi info` output from an 8×910B1
// box (npu-smi 23.0.rc2.2): Memory-Usage reports 0/0 and HBM-Usage reports
// the real 65536 MiB pool per NPU.
const npuSMI910B1 = `[ma-user work]$npu-smi info
+------------------------------------------------------------------------------------------------+
| npu-smi 23.0.rc2.2               Version: 23.0.rc2.2                                           |
+---------------------------+---------------+----------------------------------------------------+
| NPU   Name                | Health        | Power(W)    Temp(C)           Hugepages-Usage(page)|
| Chip                      | Bus-Id        | AICore(%)   Memory-Usage(MB)  HBM-Usage(MB)        |
+===========================+===============+====================================================+
| 0     910B1               | OK            | 271.1       41                0    / 0             |
| 0                         | 0000:C1:00.0  | 55          0    / 0          65099/ 65536         |
+===========================+===============+====================================================+
| 1     910B1               | OK            | 275.0       42                0    / 0             |
| 0                         | 0000:01:00.0  | 66          0    / 0          65098/ 65536         |
+===========================+===============+====================================================+
| 2     910B1               | OK            | 176.6       37                0    / 0             |
| 0                         | 0000:C2:00.0  | 96          0    / 0          65098/ 65536         |
+===========================+===============+====================================================+
| 3     910B1               | OK            | 153.2       39                0    / 0             |
| 0                         | 0000:02:00.0  | 94          0    / 0          65097/ 65536         |
+===========================+===============+====================================================+
| 4     910B1               | OK            | 139.0       35                0    / 0             |
| 0                         | 0000:81:00.0  | 96          0    / 0          65098/ 65536         |
+===========================+===============+====================================================+
| 5     910B1               | OK            | 171.1       40                0    / 0             |
| 0                         | 0000:41:00.0  | 62          0    / 0          65098/ 65536         |
+===========================+===============+====================================================+
| 6     910B1               | OK            | 196.6       36                0    / 0             |
| 0                         | 0000:82:00.0  | 55          0    / 0          65098/ 65536         |
+===========================+===============+====================================================+
| 7     910B1               | OK            | 210.4       38                0    / 0             |
| 0                         | 0000:42:00.0  | 85          0    / 0          65096/ 65536         |
+===========================+===============+====================================================+
`

// npuSMI910PremiumA is a verbatim captured `npu-smi info` output from an
// 8×910PremiumA box (npu-smi 23.0.rc2): Memory-Usage reports a 15137 MiB pool
// and HBM-Usage the larger 32768 MiB accelerator pool; a trailing per-process
// table follows the card table.
const npuSMI910PremiumA = `[ma-user work]$npu-smi info
+------------------------------------------------------------------------------------------------+
| npu-smi 23.0.rc2                 Version: 23.0.rc2                                             |
+---------------------------+---------------+----------------------------------------------------+
| NPU   Name                | Health        | Power(W)    Temp(C)           Hugepages-Usage(page)|
| Chip                      | Bus-Id        | AICore(%)   Memory-Usage(MB)  HBM-Usage(MB)        |
+===========================+===============+====================================================+
| 0     910PremiumA         | OK            | 143.7       39                34   / 34            |
| 0                         | 0000:C1:00.0  | 11          1413 / 15137      31842/ 32768         |
+===========================+===============+====================================================+
| 1     910PremiumA         | OK            | 295.7       41                34   / 34            |
| 0                         | 0000:81:00.0  | 90          1612 / 15137      31842/ 32768         |
+===========================+===============+====================================================+
| 2     910PremiumA         | OK            | 138.8       42                34   / 34            |
| 0                         | 0000:41:00.0  | 87          3161 / 15137      31842/ 32768         |
+===========================+===============+====================================================+
| 3     910PremiumA         | OK            | 179.6       41                34   / 34            |
| 0                         | 0000:01:00.0  | 15          2436 / 15039      31842/ 32768         |
+===========================+===============+====================================================+
| 4     910PremiumA         | OK            | 93.3        41                34   / 34            |
| 0                         | 0000:C2:00.0  | 46          1537 / 15137      31842/ 32768         |
+===========================+===============+====================================================+
| 5     910PremiumA         | OK            | 93.1        44                34   / 34            |
| 0                         | 0000:82:00.0  | 65          1940 / 15137      31842/ 32768         |
+===========================+===============+====================================================+
| 6     910PremiumA         | OK            | 183.6       44                34   / 34            |
| 0                         | 0000:42:00.0  | 71          2946 / 15137      31842/ 32768         |
+===========================+===============+====================================================+
| 7     910PremiumA         | OK            | 189.2       41                34   / 34            |
| 0                         | 0000:02:00.0  | 71          2197 / 15039      31842/ 32768         |
+===========================+===============+====================================================+
+---------------------------+---------------+----------------------------------------------------+
| NPU     Chip              | Process id    | Process name             | Process memory(MB)      |
+===========================+===============+====================================================+
| 0       0                 | 5139          | python                   | 31913                   |
+===========================+===============+====================================================+
| 1       0                 | 5141          | python                   | 31913                   |
+===========================+===============+====================================================+
| 2       0                 | 5143          | python                   | 31913                   |
+===========================+===============+====================================================+
| 3       0                 | 5145          | python                   | 31913                   |
+===========================+===============+====================================================+
| 4       0                 | 5147          | python                   | 31912                   |
+===========================+===============+====================================================+
| 5       0                 | 5149          | python                   | 31913                   |
+===========================+===============+====================================================+
| 6       0                 | 5151          | python                   | 31912                   |
+===========================+===============+====================================================+
| 7       0                 | 5153          | python                   | 31912                   |
+===========================+===============+====================================================+
`

const mib = uint64(1024 * 1024)

func TestParseNPUSMITable_910B1(t *testing.T) {
	cards := parseNPUSMITable(npuSMI910B1)
	if len(cards) != 8 {
		t.Fatalf("910B1 box: want 8 cards, got %d", len(cards))
	}
	for i, c := range cards {
		if c.Vendor != VendorHuawei || c.Model != "910B1" || c.Backend != BackendCANN {
			t.Fatalf("card %d: want huawei/910B1/cann, got %s/%s/%s", i, c.Vendor, c.Model, c.Backend)
		}
		if c.Index != i {
			t.Fatalf("card %d: want index %d, got %d", i, i, c.Index)
		}
		if c.VRAMBytes != 65536*mib {
			t.Fatalf("card %d: want 65536 MiB HBM, got %d bytes", i, c.VRAMBytes)
		}
		if c.Source != "npu-smi" {
			t.Fatalf("card %d: want source npu-smi, got %s", i, c.Source)
		}
	}
}

func TestParseNPUSMITable_910PremiumA(t *testing.T) {
	cards := parseNPUSMITable(npuSMI910PremiumA)
	if len(cards) != 8 {
		t.Fatalf("910PremiumA box: want 8 cards (process table ignored), got %d", len(cards))
	}
	for i, c := range cards {
		if c.Model != "910PremiumA" {
			t.Fatalf("card %d: want model 910PremiumA, got %s", i, c.Model)
		}
		// HBM 32768 MiB wins over the 15137 MiB Memory-Usage pool.
		if c.VRAMBytes != 32768*mib {
			t.Fatalf("card %d: want 32768 MiB (largest pool), got %d bytes", i, c.VRAMBytes)
		}
	}
}

func TestParseNPUSMITable_UnrecognizedShapes(t *testing.T) {
	for name, out := range map[string]string{
		"empty":        "",
		"garbage":      "not a table at all\njust text\n",
		"headers only": "| NPU   Name | Health |\n| Chip  | Bus-Id |\n",
		"error text":   "ERROR: driver not loaded\n",
	} {
		if cards := parseNPUSMITable(out); len(cards) != 0 {
			t.Fatalf("%s: want no fabricated cards, got %d (%+v)", name, len(cards), cards)
		}
	}
}

func TestParseMThreadsGMI(t *testing.T) {
	doc, err := json.Marshal(map[string]any{
		"gpus": []map[string]string{
			{"product_name": "MTT S80", "memory_total": "16384 MiB"},
			{"product_name": "MTT S80", "memory_total": "16384 MiB"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cards := parseMThreadsGMI(doc)
	if len(cards) != 2 {
		t.Fatalf("want 2 cards, got %d", len(cards))
	}
	for i, c := range cards {
		if c.Vendor != VendorMooreThreads || c.Model != "MTT S80" || c.Backend != BackendMUSA {
			t.Fatalf("card %d: want moorethreads/MTT S80/musa, got %s/%s/%s", i, c.Vendor, c.Model, c.Backend)
		}
		if c.VRAMBytes != 16384*mib {
			t.Fatalf("card %d: want 16384 MiB, got %d bytes", i, c.VRAMBytes)
		}
		if c.Source != "mthreads-gmi" || c.Index != i {
			t.Fatalf("card %d: want mthreads-gmi index %d, got %s/%d", i, i, c.Source, c.Index)
		}
	}
}

func TestParseMThreadsGMI_UnrecognizedShapes(t *testing.T) {
	for name, data := range map[string]string{
		"broken json": `{"gpus": [`,
		"wrong shape": `{"cards": [{"product_name": "MTT S80"}]}`,
		"no gpus":     `{"gpus": []}`,
		"no name":     `{"gpus": [{"memory_total": "16384 MiB"}]}`,
	} {
		if cards := parseMThreadsGMI([]byte(data)); len(cards) != 0 {
			t.Fatalf("%s: want no cards, got %d", name, len(cards))
		}
	}
}

// writeFakeCLI installs a shell script named name into dir that prints
// stdout and exits 0, so an end-to-end probe test can drive the real
// LookPath + exec path with fixture output. /bin/cat is invoked by absolute
// path because these tests replace PATH entirely (isolating them from any
// real vendor CLI on the host); the shebang interpreter is resolved by the
// kernel, not via PATH.
func writeFakeCLI(t *testing.T, dir, name, stdout string) {
	t.Helper()
	script := "#!/bin/sh\n/bin/cat <<'HWCFGMAP_EOF'\n" + stdout + "\nHWCFGMAP_EOF\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestProbeCNCards_FakeNPUSMI is the end-to-end repro of the initial-release
// stub gap: with a (fixture-printing) npu-smi on PATH, it reported vram_bytes 0 and
// the synthesizer emitted a CPU-only line; now the table is parsed into real
// per-NPU VRAM.
func TestProbeCNCards_FakeNPUSMI(t *testing.T) {
	dir := t.TempDir()
	writeFakeCLI(t, dir, "npu-smi", npuSMI910B1)
	t.Setenv("PATH", dir)

	cards, err := ProbeCNCards()
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 8 {
		t.Fatalf("want 8 parsed Ascend cards, got %d (%+v)", len(cards), cards)
	}
	if cards[0].VRAMBytes != 65536*mib || cards[0].Backend != BackendCANN {
		t.Fatalf("card 0: want 64 GiB cann, got %+v", cards[0])
	}
}

func TestProbeCNCards_FakeMThreadsGMI(t *testing.T) {
	dir := t.TempDir()
	writeFakeCLI(t, dir, "mthreads-gmi", `{"gpus": [{"product_name": "MTT S80", "memory_total": "16384 MiB"}]}`)
	t.Setenv("PATH", dir)

	cards, err := ProbeCNCards()
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("want 1 parsed Moore Threads card, got %d (%+v)", len(cards), cards)
	}
	if cards[0].Model != "MTT S80" || cards[0].VRAMBytes != 16384*mib || cards[0].Backend != BackendMUSA {
		t.Fatalf("want MTT S80 / 16 GiB / musa, got %+v", cards[0])
	}
}

// Without mthreads-gmi, a runnable mthreads-smi still reports the card with
// an honest zero VRAM (presence stub).
func TestProbeCNCards_MThreadsSMIFallback(t *testing.T) {
	dir := t.TempDir()
	writeFakeCLI(t, dir, "mthreads-smi", "MTT S80 | 100 MiB / 16384 MiB")
	t.Setenv("PATH", dir)

	cards, err := ProbeCNCards()
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("want 1 presence card, got %d (%+v)", len(cards), cards)
	}
	if cards[0].Backend != BackendMUSA || cards[0].VRAMBytes != 0 {
		t.Fatalf("want musa backend with honest zero VRAM, got %+v", cards[0])
	}
}

// A runnable npu-smi whose output is not the documented table degrades to
// the presence stub — the card is reported, VRAM stays an honest zero.
func TestProbeCNCards_NPUSMIUnrecognizedFallsBackToStub(t *testing.T) {
	dir := t.TempDir()
	writeFakeCLI(t, dir, "npu-smi", "some future npu-smi format")
	t.Setenv("PATH", dir)

	cards, err := ProbeCNCards()
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("want 1 stub card, got %d (%+v)", len(cards), cards)
	}
	if cards[0].Vendor != VendorHuawei || cards[0].VRAMBytes != 0 || cards[0].Backend != BackendCANN {
		t.Fatalf("want huawei/cann stub with zero VRAM, got %+v", cards[0])
	}
}

func TestPrimaryBackend(t *testing.T) {
	cases := []struct {
		name string
		bp   BoxProfile
		want string
	}{
		{"no cards", BoxProfile{}, BackendCPU},
		{"single cuda", BoxProfile{GPU: []GPUCard{{Vendor: VendorNVIDIA, VRAMBytes: 16 * mib, Backend: BackendCUDA}}}, BackendCUDA},
		{"largest wins", BoxProfile{GPU: []GPUCard{
			{Vendor: VendorNVIDIA, VRAMBytes: 8 * mib, Backend: BackendCUDA, Index: 0},
			{Vendor: VendorHuawei, VRAMBytes: 64 * mib, Backend: BackendCANN, Index: 1},
		}}, BackendCANN},
		{"zero-vram card still reports backend", BoxProfile{GPU: []GPUCard{{Vendor: VendorBiren, VRAMBytes: 0, Backend: BackendUnknown}}}, BackendUnknown},
		{"empty backend maps to unknown", BoxProfile{GPU: []GPUCard{{Vendor: VendorBiren, VRAMBytes: 0}}}, BackendUnknown},
	}
	for _, tc := range cases {
		if got := tc.bp.PrimaryBackend(); got != tc.want {
			t.Fatalf("%s: want %s, got %s", tc.name, tc.want, got)
		}
	}
}
