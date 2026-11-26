//go:build !nsgo_noshader

package gpu

import (
	"errors"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/shaderir"
)

func TestTranslateShaderModuleAndArtifacts(t *testing.T) {
	if _, err := TranslateShaderModule(nil); err == nil {
		t.Fatal("TranslateShaderModule(nil) = nil, want error")
	}

	renderMod := MustCompileWGSL(`
struct Uniforms {
	mvp: mat4x4<f32>,
};
@group(0) @binding(0) var<uniform> u: Uniforms;

struct VSOut {
	@builtin(position) pos: vec4<f32>,
	@location(0) color: vec4<f32>,
};

@vertex
fn vs_main(@location(0) pos: vec3<f32>, @location(1) color: vec4<f32>) -> VSOut {
	var out: VSOut;
	out.pos = u.mvp * vec4<f32>(pos, 1.0);
	out.color = color;
	return out;
}

@fragment
fn fs_main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
	return color;
}
`)
	ps, err := TranslateShaderModule(renderMod)
	if err != nil {
		t.Fatalf("TranslateShaderModule(render) error: %v", err)
	}
	if ps.MSL == "" || ps.HLSLVertex == "" || ps.HLSLPixel == "" ||
		ps.GLSLVertex == "" || ps.GLSLFragment == "" ||
		ps.GLESVertex == "" || ps.GLESFragment == "" || ps.WGSL == "" {
		t.Fatalf("missing expected translated sources: %+v", ps)
	}
	if ps.MSLError != nil || ps.HLSLError != nil || ps.GLSLError != nil || ps.GLESError != nil || ps.WGSLError != nil {
		t.Fatalf("unexpected backend errors: %+v", ps)
	}

	// Storage buffer render shader succeeds on MSL, HLSL, WGSL and reports GLSL/GLES errors.
	storageMod := MustCompileWGSL(`
@group(0) @binding(0) var<storage, read> colors: array<vec4<f32>>;

struct VSOut {
	@builtin(position) pos: vec4<f32>,
	@location(0) color: vec4<f32>,
};

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> VSOut {
	var out: VSOut;
	out.pos = vec4<f32>(0.0, 0.0, 0.0, 1.0);
	out.color = colors[vid];
	return out;
}

@fragment
fn fs_main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
	return color;
}
`)
	sPS, err := TranslateShaderModule(storageMod)
	if err == nil {
		t.Fatal("TranslateShaderModule(storage) = nil, want GLSL/GLES errors")
	}
	if sPS.MSL == "" || sPS.HLSLVertex == "" || sPS.HLSLPixel == "" || sPS.WGSL == "" {
		t.Errorf("expected MSL, HLSL, WGSL sources, got %+v", sPS)
	}
	if sPS.MSLError != nil || sPS.HLSLError != nil || sPS.WGSLError != nil {
		t.Errorf("unexpected errors on MSL/HLSL/WGSL: msl=%v hlsl=%v wgsl=%v", sPS.MSLError, sPS.HLSLError, sPS.WGSLError)
	}
	if sPS.GLSLError == nil || !strings.Contains(sPS.GLSLError.Error(), "GLSL") {
		t.Errorf("expected GLSLError naming GLSL, got %v", sPS.GLSLError)
	}
	if sPS.GLESError == nil || !strings.Contains(sPS.GLESError.Error(), "GLES") {
		t.Errorf("expected GLESError naming GLES, got %v", sPS.GLESError)
	}

	// Kage fragment module translates cleanly when self-contained.
	kageMod, err := CompileKage(`package main

func Fragment(dst vec4, pos vec2) vec4 {
	return vec4(1.0, 0.0, 0.0, 1.0)
}
`)
	if err != nil {
		t.Fatalf("CompileKage: %v", err)
	}
	kPS, err := TranslateShaderModule(kageMod)
	if err != nil {
		t.Fatalf("TranslateShaderModule(kage) error: %v", err)
	}
	if kPS.MSL == "" || kPS.HLSLPixel == "" || kPS.GLSLFragment == "" || kPS.GLESFragment == "" || kPS.WGSL == "" {
		t.Errorf("expected all fragment backends for Kage module, got %+v", kPS)
	}

	// compilePipelineArtifacts still succeeds when at least one backend compiles,
	// and returns an error instead of empty artifacts when every backend fails.
	art, err := compilePipelineArtifacts(driver.GPUPipelineDesc{
		Vertex:   driver.GPUShaderStage{IR: storageMod.ir},
		Fragment: driver.GPUShaderStage{IR: storageMod.ir},
	})
	if err != nil {
		t.Fatalf("compilePipelineArtifacts(storage) unexpected error: %v", err)
	}
	if art.MSLSource == "" || art.HLSLVertexSource == "" || art.WGSLSource == "" {
		t.Errorf("expected partial artifacts for storage shader, got %+v", art)
	}

	if _, err := compilePipelineArtifacts(driver.GPUPipelineDesc{
		Vertex:   driver.GPUShaderStage{IR: &shaderir.Program{}},
		Fragment: driver.GPUShaderStage{IR: &shaderir.Program{}},
	}); err == nil {
		t.Fatal("compilePipelineArtifacts(empty IR) = nil, want error")
	}

	// Panic recovery in pipeline translation turns translator panics into errors naming every failing backend.
	panicIR := &shaderir.Program{
		Bindings: []shaderir.Binding{{
			Kind: shaderir.BindingUniformBuffer,
			Type: shaderir.Type{Main: shaderir.Array},
		}},
		VertexFunc: shaderir.VertexFunc{Block: &shaderir.Block{}},
	}
	_, pPS := translatePipelineIR(panicIR)
	for name, bErr := range map[string]error{
		"MSL":  pPS.MSLError,
		"HLSL": pPS.HLSLError,
		"GLSL": pPS.GLSLError,
		"GLES": pPS.GLESError,
		"WGSL": pPS.WGSLError,
	} {
		if bErr == nil || !strings.Contains(bErr.Error(), name) || !strings.Contains(bErr.Error(), "panic:") {
			t.Errorf("%s panic error = %v, want recovered panic naming %s", name, bErr, name)
		}
	}

	// Compute shader succeeds on MSL, HLSL, WGSL and reports GLSL/GLES errors.
	computeMod := MustCompileWGSL(`
@group(0) @binding(0) var<storage, read_write> data: array<f32>;

@compute @workgroup_size(4)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
	data[gid.x] = data[gid.x] * 2.0;
}
`)
	cPS, err := TranslateShaderModule(computeMod)
	if err == nil {
		t.Fatal("TranslateShaderModule(compute) = nil, want GLSL/GLES errors")
	}
	if cPS.MSL == "" || cPS.HLSLCompute == "" || cPS.WGSL == "" {
		t.Errorf("expected MSL, HLSLCompute, WGSL sources for compute shader, got %+v", cPS)
	}
	if cPS.GLSLCompute != "" || cPS.GLESCompute != "" {
		t.Errorf("expected empty GLSLCompute/GLESCompute for compute shader, got GLSL=%q GLES=%q", cPS.GLSLCompute, cPS.GLESCompute)
	}
	if cPS.MSLError != nil || cPS.HLSLError != nil || cPS.WGSLError != nil {
		t.Errorf("unexpected compute errors on MSL/HLSL/WGSL: msl=%v hlsl=%v wgsl=%v", cPS.MSLError, cPS.HLSLError, cPS.WGSLError)
	}
	if cPS.GLSLError == nil || !strings.Contains(cPS.GLSLError.Error(), "GLSL") {
		t.Errorf("expected GLSLError naming GLSL, got %v", cPS.GLSLError)
	}
	if cPS.GLESError == nil || !strings.Contains(cPS.GLESError.Error(), "GLES") {
		t.Errorf("expected GLESError naming GLES, got %v", cPS.GLESError)
	}
}

func TestCreatePipelineAsyncAndCacheReportCompileErrors(t *testing.T) {
	panicIR := &shaderir.Program{
		Bindings: []shaderir.Binding{{
			Kind: shaderir.BindingUniformBuffer,
			Type: shaderir.Type{Main: shaderir.Array},
		}},
		VertexFunc: shaderir.VertexFunc{Block: &shaderir.Block{}},
	}
	badMod := &ShaderModule{
		ir:            panicIR,
		vertexEntries: []string{"vs_main"},
	}

	sr := NewSoftwareRenderer(32, 32, FormatUndefined)
	dev := sr.Device()

	var readyPipe *Pipeline
	var readyErr error
	pipe := dev.CreatePipelineAsync(nil, PipelineDesc{
		Label:   "bad-async",
		Vertex:  ShaderStage{Module: badMod, EntryPoint: "vs_main"},
		Targets: []ColorTargetState{{Format: dev.SurfaceFormat()}},
	}, func(p *Pipeline, err error) {
		readyPipe = p
		readyErr = err
	})
	if pipe == nil {
		t.Fatal("CreatePipelineAsync returned nil *Pipeline")
	}
	<-pipe.Done()

	if readyPipe != nil {
		t.Errorf("onReady pipeline = %v, want nil", readyPipe)
	}
	if readyErr == nil {
		t.Fatal("onReady err = nil, want ErrShaderCompile")
	}
	var gerr Error
	if !errors.As(readyErr, &gerr) || gerr.Code != ErrShaderCompile {
		t.Errorf("onReady err = %v, want Error{Code: ErrShaderCompile}", readyErr)
	}
	if !strings.Contains(readyErr.Error(), "panic:") {
		t.Errorf("onReady err = %q, want recovered panic details", readyErr.Error())
	}
	if pipe.Err() == nil {
		t.Error("pipe.Err() = nil, want compile error")
	}
	if pipe.Ready() {
		t.Error("pipe.Ready() = true after compile failure, want false")
	}

	cache, err := NewPipelineCache(t.TempDir())
	if err != nil {
		t.Fatalf("NewPipelineCache: %v", err)
	}
	dev.SetPipelineCache(cache)
	if _, err := dev.CreatePipeline(nil, PipelineDesc{
		Label:   "bad-cached",
		Vertex:  ShaderStage{Module: badMod, EntryPoint: "vs_main"},
		Targets: []ColorTargetState{{Format: dev.SurfaceFormat()}},
	}); !errors.As(err, &gerr) || gerr.Code != ErrShaderCompile {
		t.Errorf("CreatePipeline with PipelineCache err = %v, want Error{Code: ErrShaderCompile}", err)
	}
}
