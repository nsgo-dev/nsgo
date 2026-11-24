package gpu_test

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/gpu"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestRenderPassRejectedBindings(t *testing.T) {
	const testID = "render-bindings"
	const selector = "id:" + testID
	const otherID = "foreign-device"
	const otherSelector = "id:" + otherID
	var dev, foreign *gpu.Device
	var gpuErrors []gpu.Error
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		style := core.Style{Width: core.Pt(8), Height: core.Pt(8)}
		return core.View{Children: []core.Node{
			gpu.View(cx, gpu.ViewProps{
				TestID:  testID,
				Style:   style,
				OnReady: func(d *gpu.Device) { dev = d },
				OnError: func(err gpu.Error) { gpuErrors = append(gpuErrors, err) },
			}),
			gpu.View(cx, gpu.ViewProps{
				TestID:  otherID,
				Style:   style,
				OnReady: func(d *gpu.Device) { foreign = d },
			}),
		}}
	}, nsgotest.Size(16, 16), nsgotest.ManualFrames())
	if dev == nil || foreign == nil {
		t.Fatal("headless GPU devices did not become ready")
	}

	vertexData := packFloat32s(0, 0, 0, 0.5, 0, 0, 0, 0.5, 0)
	indexData := packUint16s(0, 1, 2, 0)
	buffer := func(d *gpu.Device, usage gpu.BufferUsage, data []byte) *gpu.Buffer {
		t.Helper()
		buf, err := d.CreateBuffer(nil, gpu.BufferDesc{Usage: usage, Data: data})
		if err != nil {
			t.Fatal(err)
		}
		return buf
	}

	staleVertex := buffer(dev, gpu.UsageVertex, vertexData)
	staleIndex := buffer(dev, gpu.UsageIndex, indexData)
	app.TriggerGPUDeviceLost(selector, nsgotest.GPULostReset, "test reset")
	app.RestoreGPUDevice(selector)
	app.TriggerGPUDeviceLost(otherSelector, nsgotest.GPULostReset, "test reset")
	app.RestoreGPUDevice(otherSelector)
	if foreign == dev || foreign.Epoch() != dev.Epoch() {
		t.Fatal("expected two distinct devices at the same epoch")
	}
	if staleVertex.Valid() || staleIndex.Valid() || dev.Epoch() == staleVertex.Epoch() {
		t.Fatal("device reset did not leave the old buffers at an earlier epoch")
	}

	// Use distinct, correctly sized buffers so even an erroneously recorded draw
	// in these checks never reads out of range.
	vertex := buffer(dev, gpu.UsageVertex, vertexData)
	colors := buffer(dev, gpu.UsageVertex, packFloat32s(
		1, 0, 0, 1, 0, 1, 0, 1, 0, 0, 1, 1,
	))
	index := buffer(dev, gpu.UsageIndex, indexData)
	closedVertex := buffer(dev, gpu.UsageVertex, vertexData)
	closedIndex := buffer(dev, gpu.UsageIndex, indexData)
	if err := closedVertex.Close(); err != nil {
		t.Fatal(err)
	}
	if err := closedIndex.Close(); err != nil {
		t.Fatal(err)
	}
	foreignVertex := buffer(foreign, gpu.UsageVertex, vertexData)
	foreignIndex := buffer(foreign, gpu.UsageIndex, indexData)

	uniform := buffer(dev, gpu.UsageUniform, packFloat32s(
		1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1,
	))
	layout, err := dev.CreateBindGroupLayout(nil, []gpu.BindGroupLayoutEntry{
		{Binding: 0, Visibility: gpu.StageVertex, Type: gpu.BindingUniform},
	})
	if err != nil {
		t.Fatal(err)
	}
	group, err := dev.CreateBindGroup(nil, layout, []gpu.BindGroupEntry{{Binding: 0, Buffer: uniform}})
	if err != nil {
		t.Fatal(err)
	}
	mod := gpu.MustCompileWGSL(color3DWGSL)
	desc := gpu.PipelineDesc{
		Vertex:   gpu.ShaderStage{Module: mod, EntryPoint: "vs_main"},
		Fragment: gpu.ShaderStage{Module: mod, EntryPoint: "fs_main"},
		VertexBuffers: []gpu.VertexBufferLayout{
			{Stride: 12, Attributes: []gpu.VertexAttribute{{Location: 0, Format: gpu.VertexFloat32x3}}},
			{Stride: 16, Attributes: []gpu.VertexAttribute{{Location: 1, Format: gpu.VertexFloat32x4}}},
		},
		Targets: []gpu.ColorTargetState{{Format: dev.SurfaceFormat()}},
	}
	pipeline := func(desc gpu.PipelineDesc) *gpu.Pipeline {
		t.Helper()
		pipe, err := dev.CreatePipeline(nil, desc)
		if err != nil {
			t.Fatal(err)
		}
		return pipe
	}
	autoLayout := pipeline(desc)
	desc.Layouts = []*gpu.BindGroupLayout{layout}
	pipe := pipeline(desc)
	procedural := gpu.MustCompileWGSL(`
@vertex fn vs_main() -> @builtin(position) vec4<f32> {
	return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
@fragment fn fs_main() -> @location(0) vec4<f32> {
	return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}`)
	noBindings := pipeline(gpu.PipelineDesc{
		Vertex:   gpu.ShaderStage{Module: procedural, EntryPoint: "vs_main"},
		Fragment: gpu.ShaderStage{Module: procedural, EntryPoint: "fs_main"},
		Targets:  desc.Targets,
	})

	checkFrame := func(name string, draws int, instances uint64, record func(*gpu.RenderPass), markers ...string) {
		t.Helper()
		framesBefore := len(app.GPUFrames())
		eventsBefore := len(app.GPUDebugEvents(selector))
		app.Post(func() {
			enc := dev.BeginEncoder()
			pass := enc.BeginRenderPass(gpu.RenderPassDesc{})
			record(pass)
			pass.End()
			dev.Submit(enc.Finish())
		})
		app.Settle()
		if len(gpuErrors) != 0 {
			t.Fatalf("%s: unexpected backend GPU errors: %v", name, gpuErrors)
		}
		frames := app.GPUFrames()[framesBefore:]
		if len(frames) != 1 || frames[0].PassesCount != 1 || frames[0].DrawsCount != draws || frames[0].InstancesCount != instances {
			t.Fatalf("%s: got frames %+v, want a single pass with %d draws and %d instances", name, frames, draws, instances)
		}
		var got []string
		for _, event := range app.GPUDebugEvents(selector)[eventsBefore:] {
			if event.Kind == "pass_marker" {
				got = append(got, event.Label)
			}
		}
		if !gpu.DebugLabelsEnabled {
			markers = nil
		}
		if !slices.Equal(got, markers) {
			t.Errorf("%s: debug markers = %q, want %q", name, got, markers)
		}
	}
	bind := func(pass *gpu.RenderPass) {
		pass.SetPipeline(pipe)
		pass.SetVertexBuffer(0, vertex, 0)
		pass.SetVertexBuffer(1, colors, 0)
		pass.SetBindGroup(0, group)
	}
	drawAll := func(pass *gpu.RenderPass) {
		pass.Draw(3, 0)
		pass.DrawInstanced(3, 2, 0, 0)
		pass.DrawIndexed(3, 0, 0)
		pass.DrawIndexedInstanced(3, 2, 0, 0, 0)
	}

	// Keep the app and its frame steps on the test goroutine.
	checkFrame("no index bound", 2, 3, func(pass *gpu.RenderPass) {
		bind(pass)
		drawAll(pass)
	}, "gpu.draw_skipped operation=DrawIndexed binding=index_buffer slot=0",
		"gpu.draw_skipped operation=DrawIndexedInstanced binding=index_buffer slot=0")
	for _, tc := range []struct {
		name          string
		vertex, index *gpu.Buffer
	}{
		{"nil", nil, nil},
		{"closed", closedVertex, closedIndex},
		{"foreign", foreignVertex, foreignIndex},
		{"stale epoch", staleVertex, staleIndex},
	} {
		checkFrame(tc.name+" vertex", 5, 7, func(pass *gpu.RenderPass) {
			bind(pass)
			pass.SetIndexBuffer(index, gpu.IndexUint16, 0)
			pass.Draw(3, 0)
			pass.SetVertexBuffer(0, tc.vertex, 0)
			drawAll(pass)
			pass.SetVertexBuffer(1, colors, 0) // Only this slot is repaired.
			pass.Draw(3, 0)
			pass.SetVertexBuffer(0, vertex, 0)
			drawAll(pass)
		}, "gpu.bind_rejected operation=SetVertexBuffer slot=0",
			"gpu.draw_skipped operation=Draw binding=vertex_buffer slot=0",
			"gpu.draw_skipped operation=DrawInstanced binding=vertex_buffer slot=0",
			"gpu.draw_skipped operation=DrawIndexed binding=vertex_buffer slot=0",
			"gpu.draw_skipped operation=DrawIndexedInstanced binding=vertex_buffer slot=0",
			"gpu.draw_skipped operation=Draw binding=vertex_buffer slot=0")
		checkFrame(tc.name+" index", 5, 7, func(pass *gpu.RenderPass) {
			bind(pass)
			pass.SetIndexBuffer(index, gpu.IndexUint16, 0)
			pass.DrawIndexed(3, 0, 0)
			pass.SetIndexBuffer(tc.index, gpu.IndexUint16, 0)
			drawAll(pass) // Non-indexed draws still succeed.
			pass.SetVertexBuffer(0, vertex, 0)
			pass.DrawIndexed(3, 0, 0)
			pass.SetIndexBuffer(index, gpu.IndexUint16, 0)
			pass.DrawIndexed(3, 0, 0)
			pass.DrawIndexedInstanced(3, 2, 0, 0, 0)
		}, "gpu.bind_rejected operation=SetIndexBuffer slot=0",
			"gpu.draw_skipped operation=DrawIndexed binding=index_buffer slot=0",
			"gpu.draw_skipped operation=DrawIndexedInstanced binding=index_buffer slot=0",
			"gpu.draw_skipped operation=DrawIndexed binding=index_buffer slot=0")
	}

	checkFrame("active pipeline slots", 4, 4, func(pass *gpu.RenderPass) {
		bind(pass)
		pass.SetVertexBuffer(3, nil, 0) // Neither slot 3 is used by pipe.
		pass.SetBindGroup(3, nil)
		pass.Draw(3, 0)
		pass.SetVertexBuffer(0, nil, 0)
		pass.SetBindGroup(0, nil)
		pass.SetPipeline(noBindings)
		pass.Draw(3, 0)
		pass.SetPipeline(pipe)
		pass.Draw(3, 0)
		pass.SetVertexBuffer(0, vertex, 0)
		pass.Draw(3, 0) // Group 0 is still rejected.
		pass.SetPipeline(autoLayout)
		pass.Draw(3, 0) // The shader's inferred group 0 is also affected.
		pass.SetBindGroup(0, group)
		pass.Draw(3, 0)
		pass.SetPipeline(pipe)
		pass.Draw(3, 0)
	}, "gpu.bind_rejected operation=SetVertexBuffer slot=3",
		"gpu.bind_rejected operation=SetBindGroup slot=3",
		"gpu.bind_rejected operation=SetVertexBuffer slot=0",
		"gpu.bind_rejected operation=SetBindGroup slot=0",
		"gpu.draw_skipped operation=Draw binding=vertex_buffer slot=0",
		"gpu.draw_skipped operation=Draw binding=bind_group slot=0",
		"gpu.draw_skipped operation=Draw binding=bind_group slot=0")
}
