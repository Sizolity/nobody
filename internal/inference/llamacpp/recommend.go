package llamacpp

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Recommendation is the per-machine hint returned by Recommend(). All
// numeric fields are populated; Source distinguishes the path that
// produced the values so harness can stamp the inference_recommend_applied
// event payload meaningfully.
type Recommendation struct {
	NGL    int    // 0 = CPU-only inference; positive = GPU offload layer count
	Ctx    int    // server -c (context length) suggestion
	VRAM   int    // detected VRAM (MiB) of the first GPU; 0 when cpu-fallback
	Source string // "nvidia-smi" | "cpu-fallback" | "nvidia-smi-failed"
}

// errNvidiaSmiNotFound is the sentinel returned by lookupNvidiaSmi when
// the binary is absent from PATH. Tests override lookupNvidiaSmi to
// inject this without manipulating the real $PATH.
//
// NOTE: kept for test clarity; production code only checks `err != nil`,
// not via errors.Is, so the specific identity is not load-bearing. If
// you ever delete the seam, you can also delete this var.
var errNvidiaSmiNotFound = errors.New("nvidia-smi not in PATH")

// CONCURRENCY: tests that mutate this var must NOT call t.Parallel().
// Concurrent reassignment from multiple goroutines is a data race.
// See recommend_test.go for the defer-restore pattern.
//
// lookupNvidiaSmi resolves the absolute path of nvidia-smi, returning
// errNvidiaSmiNotFound when missing. Indirection lets tests override
// without touching exec.LookPath.
var lookupNvidiaSmi = func() (string, error) {
	p, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return "", errNvidiaSmiNotFound
	}
	return p, nil
}

// CONCURRENCY: tests that mutate this var must NOT call t.Parallel().
// Concurrent reassignment from multiple goroutines is a data race.
// See recommend_test.go for the defer-restore pattern.
//
// queryFirstGPUVRAM runs `nvidia-smi --query-gpu=memory.total
// --format=csv,noheader,nounits` with a 5s context timeout and returns
// the first GPU's total VRAM in MiB. Returns 0 + error on any failure
// (exec error, parse failure, empty output).
var queryFirstGPUVRAM = func(smiPath string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, smiPath,
		"--query-gpu=memory.total",
		"--format=csv,noheader,nounits",
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	first := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	digits := strings.Builder{}
	for _, r := range first {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if digits.Len() == 0 {
		return 0, errors.New("nvidia-smi VRAM output had no digits")
	}
	n, err := strconv.Atoi(digits.String())
	if err != nil {
		return 0, err
	}
	return n, nil
}

// Recommend probes nvidia-smi for the first GPU's VRAM and returns the
// recommended {NGL, Ctx} pair per the decision matrix mirrored from
// scripts/recommend_llamacpp_yaml.sh. Pure: no filesystem writes, no
// nobody.yaml mutation. Caller (harness.New) is responsible for applying
// the result via presence-based defaulting on cfg.managed.{NGL,Ctx}.
//
// Decision matrix (VRAM in MiB):
//
//	>= 24576  → NGL=99 Ctx=32768  (qwen3.5-27b full GPU offload)
//	>= 16384  → NGL=99 Ctx=32768  (qwen3.5-9b large ctx)
//	>=  6144  → NGL=99 Ctx=8192   (qwen3.5-9b safe ctx; 32K OOMs per Phase 1
//	                                benchmark on RTX 5060 8 GB)
//	>=  4096  → NGL=22 Ctx=4096   (hybrid offload, partial CPU)
//	<  4096   → NGL=99 Ctx=8192   (qwen3.5-4b fits fully in low VRAM)
//	no GPU    → NGL=0  Ctx=8192   (cpu-fallback, qwen3.5-4b CPU)
//	query failed → NGL=0  Ctx=8192   (nvidia-smi-failed; binary present but
//	                                  output empty / non-numeric / VRAM<=0 /
//	                                  query timed out)
func Recommend() Recommendation {
	smi, err := lookupNvidiaSmi()
	if err != nil {
		return Recommendation{NGL: 0, Ctx: 8192, VRAM: 0, Source: "cpu-fallback"}
	}
	vram, err := queryFirstGPUVRAM(smi)
	if err != nil || vram <= 0 {
		return Recommendation{NGL: 0, Ctx: 8192, VRAM: 0, Source: "nvidia-smi-failed"}
	}
	switch {
	case vram >= 24576:
		return Recommendation{NGL: 99, Ctx: 32768, VRAM: vram, Source: "nvidia-smi"}
	case vram >= 16384:
		return Recommendation{NGL: 99, Ctx: 32768, VRAM: vram, Source: "nvidia-smi"}
	case vram >= 6144:
		return Recommendation{NGL: 99, Ctx: 8192, VRAM: vram, Source: "nvidia-smi"}
	case vram >= 4096:
		return Recommendation{NGL: 22, Ctx: 4096, VRAM: vram, Source: "nvidia-smi"}
	default:
		return Recommendation{NGL: 99, Ctx: 8192, VRAM: vram, Source: "nvidia-smi"}
	}
}
