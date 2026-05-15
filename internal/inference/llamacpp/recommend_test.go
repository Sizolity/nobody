package llamacpp

import (
	"errors"
	"strings"
	"testing"
)

func TestRecommend_CPUFallbackWhenSmiMissing(t *testing.T) {
	orig := lookupNvidiaSmi
	defer func() { lookupNvidiaSmi = orig }()
	lookupNvidiaSmi = func() (string, error) {
		return "", errNvidiaSmiNotFound
	}
	rec := Recommend()
	if rec.NGL != 0 {
		t.Errorf("NGL = %d, want 0 (cpu-fallback)", rec.NGL)
	}
	if rec.Ctx != 8192 {
		t.Errorf("Ctx = %d, want 8192 (cpu-fallback)", rec.Ctx)
	}
	if rec.VRAM != 0 {
		t.Errorf("VRAM = %d, want 0 (no GPU detected)", rec.VRAM)
	}
	if !strings.Contains(rec.Source, "cpu-fallback") {
		t.Errorf("Source = %q, want to contain cpu-fallback", rec.Source)
	}
}

func TestRecommend_VRAMBranches(t *testing.T) {
	cases := []struct {
		name    string
		vram    int
		wantNGL int
		wantCtx int
	}{
		{name: "27B-class >=24G", vram: 24576, wantNGL: 99, wantCtx: 32768},
		{name: "9B-large >=16G", vram: 16384, wantNGL: 99, wantCtx: 32768},
		{name: "9B-safe >=6G", vram: 8192, wantNGL: 99, wantCtx: 8192},
		{name: "hybrid 4-5G", vram: 4096, wantNGL: 22, wantCtx: 4096},
		{name: "4B-fit <4G", vram: 2048, wantNGL: 99, wantCtx: 8192},
	}
	origLookup := lookupNvidiaSmi
	origQuery := queryFirstGPUVRAM
	defer func() {
		lookupNvidiaSmi = origLookup
		queryFirstGPUVRAM = origQuery
	}()
	lookupNvidiaSmi = func() (string, error) { return "/usr/bin/nvidia-smi", nil }
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vram := tc.vram
			queryFirstGPUVRAM = func(string) (int, error) { return vram, nil }
			rec := Recommend()
			if rec.NGL != tc.wantNGL {
				t.Errorf("NGL = %d, want %d", rec.NGL, tc.wantNGL)
			}
			if rec.Ctx != tc.wantCtx {
				t.Errorf("Ctx = %d, want %d", rec.Ctx, tc.wantCtx)
			}
			if rec.VRAM != tc.vram {
				t.Errorf("VRAM = %d, want %d", rec.VRAM, tc.vram)
			}
			if rec.Source != "nvidia-smi" {
				t.Errorf("Source = %q, want nvidia-smi", rec.Source)
			}
		})
	}
}

func TestRecommend_NvidiaSmiQueryFailsFallsBackToCPU(t *testing.T) {
	origLookup := lookupNvidiaSmi
	origQuery := queryFirstGPUVRAM
	defer func() {
		lookupNvidiaSmi = origLookup
		queryFirstGPUVRAM = origQuery
	}()
	lookupNvidiaSmi = func() (string, error) { return "/usr/bin/nvidia-smi", nil }
	queryFirstGPUVRAM = func(string) (int, error) { return 0, errors.New("simulated failure") }
	rec := Recommend()
	if rec.NGL != 0 || rec.Ctx != 8192 {
		t.Errorf("got NGL=%d Ctx=%d, want 0/8192 (cpu fallback)", rec.NGL, rec.Ctx)
	}
	if rec.Source != "nvidia-smi-failed" {
		t.Errorf("Source = %q, want nvidia-smi-failed", rec.Source)
	}
}
