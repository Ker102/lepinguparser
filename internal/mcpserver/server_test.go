package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Ker102/lepinguparser/internal/testdocx"
)

func TestMCPTooriistad(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fail := filepath.Join(dir, "leping.docx")
	if err := os.WriteFile(fail, testdocx.Koosta(testdocx.NaidisLeping(), testdocx.Nummerdus), 0o600); err != nil {
		t.Fatal(err)
	}

	st, ct := mcp.NewInMemoryTransports()
	srv := Uus("test")
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 9 {
		t.Fatalf("ootasin 9 tööriista, sain %d", len(tools.Tools))
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "kontroll", Arguments: map[string]any{"fail": fail}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tööriista viga: %+v", res.Content)
	}
	b, _ := json.Marshal(res.StructuredContent)
	var ar struct {
		KatkisedViited []struct{ Siht string } `json:"katkised_viited"`
	}
	if err := json.Unmarshal(b, &ar); err != nil {
		t.Fatal(err)
	}
	if len(ar.KatkisedViited) != 1 || ar.KatkisedViited[0].Siht != "7.4" {
		t.Fatalf("ootasin katkist viidet 7.4, sain %s", b)
	}

	// juurdepääsupiirang
	t.Setenv("LEPINGUPARSER_JUURED", filepath.Join(dir, "muu"))
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "sisukord", Arguments: map[string]any{"fail": fail}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("piiratud kaustast väljas olev fail peab andma vea")
	}
}
