package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nsgo-dev/nsgo/internal/automation"
)

// fakeBridge answers as nsgo dev -automation would, recording the calls.
type fakeBridge struct {
	calls []string
	png   []byte
}

func (f *fakeBridge) call(_ context.Context, method string, params json.RawMessage) (json.RawMessage, *automation.Error) {
	f.calls = append(f.calls, method+" "+string(params))
	var result any
	switch method {
	case automation.Screenshot:
		result = automation.ScreenshotResult{PNG: f.png, Width: 2, Height: 1}
	case automation.Press:
		var p automation.PressParams
		json.Unmarshal(params, &p)
		if p.Selector == "id:none" {
			return nil, automation.Errorf(automation.NoMatch, "no view shown matches %q", p.Selector)
		}
		result = automation.StepResult{Settled: true, Snapshot: "- button \"Add\" [view:3]"}
	default:
		result = automation.StatusResult{Target: "native", App: true, Run: 1}
	}
	data, _ := json.Marshal(result)
	return data, nil
}

// connectMCP connects a client to an nsgo mcp server answering through b.
func connectMCP(t *testing.T, b mcpBridge) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := newMCPServer(b).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestMCPTools(t *testing.T) {
	cs := connectMCP(t, &fakeBridge{})
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
		if tool.Name == automation.Press {
			schema, _ := json.Marshal(tool.InputSchema)
			if !strings.Contains(string(schema), "an nsgotest selector") || !strings.Contains(string(schema), `"required":["selector"]`) {
				t.Errorf("press's input schema: %s", schema)
			}
		}
	}
	slices.Sort(names)
	want := []string{"alert", "drag", "find", "hover", "key", "logs", "menu", "press", "reload", "screenshot", "scroll", "snapshot", "status", "type", "wait", "windows"}
	if !slices.Equal(names, want) {
		t.Errorf("tools %v, want %v", names, want)
	}
	if !strings.Contains(cs.InitializeResult().Instructions, "selector") {
		t.Error("the server tells clients nothing of selectors")
	}
}

func TestMCPResults(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.White)
	var pic bytes.Buffer
	png.Encode(&pic, img)
	b := &fakeBridge{png: pic.Bytes()}
	cs := connectMCP(t, b)
	ctx := context.Background()
	shot, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: automation.Screenshot, Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	shown, ok := shot.Content[0].(*mcp.ImageContent)
	if !ok || shown.MIMEType != "image/png" {
		t.Fatalf("screenshot %+v", shot.Content)
	}
	if cfg, err := png.DecodeConfig(bytes.NewReader(shown.Data)); err != nil || cfg.Width != 2 {
		t.Fatalf("the screenshot's image: %+v %v", cfg, err)
	}
	press, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: automation.Press, Arguments: map[string]any{"selector": "id:add", "snapshot": true}})
	if err != nil || press.IsError {
		t.Fatalf("press: %+v %v", press, err)
	}
	if text := press.Content[0].(*mcp.TextContent).Text; !strings.HasPrefix(text, "done\n\n- button") {
		t.Errorf("press answered %q", text)
	}
	if !slices.Contains(b.calls, `press {"selector":"id:add","snapshot":true}`) {
		t.Errorf("the bridge was asked %q", b.calls)
	}
	failed, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: automation.Press, Arguments: map[string]any{"selector": "id:none"}})
	if err != nil || !failed.IsError || !strings.HasPrefix(failed.Content[0].(*mcp.TextContent).Text, "no_match: ") {
		t.Fatalf("a failed press: %+v %v", failed, err)
	}
	// The schema requires a selector: the SDK refuses the call before the bridge sees it.
	n := len(b.calls)
	if r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: automation.Press, Arguments: map[string]any{}}); err == nil && !r.IsError || len(b.calls) != n {
		t.Errorf("a press without a selector was taken: %q", b.calls)
	}
}
