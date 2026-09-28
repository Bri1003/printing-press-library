package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type tbAttachFixture struct {
	home, docs, inside, outside string
}

func tbSetupAttach(t *testing.T, mcp bool) tbAttachFixture {
	t.Helper()
	b := tbSetupB(t, false, false)
	base := t.TempDir()
	f := tbAttachFixture{home: b.home, docs: filepath.Join(base, "Documents")}
	outDir := filepath.Join(base, "Outside")
	for _, d := range []string{f.docs, outDir, filepath.Join(base, "DocumentsX")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.inside = filepath.Join(f.docs, "report.txt")
	f.outside = filepath.Join(outDir, "secret.txt")
	for _, p := range []string{f.inside, f.outside, filepath.Join(base, "DocumentsX", "x.txt"), filepath.Join(f.docs, "second.txt")} {
		if err := os.WriteFile(p, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prev := tbDocumentsDir
	tbDocumentsDir = func() (string, error) { return f.docs, nil }
	t.Cleanup(func() { tbDocumentsDir = prev })
	surface := ""
	if mcp {
		surface = "mcp"
	}
	t.Setenv("THUNDERBIRD_LEARN_SURFACE", surface)
	t.Setenv(mcpBoundProfileEnv, "")
	return f
}

func tbDraftAttach(t *testing.T, f tbAttachFixture, attach string) (tbComposeSpec, string, error) {
	t.Helper()
	out, errOut, err := tbRun(t, f.home, "drafts", "new", "--to", "alice@example.com", "--attach="+attach, "--json")
	if err != nil {
		return tbComposeSpec{}, errOut + err.Error(), err
	}
	return tbDecode[tbComposeSpec](t, out), "", nil
}

func tbAssertAttachRejected(t *testing.T, f tbAttachFixture, attach string) {
	t.Helper()
	_, msg, err := tbDraftAttach(t, f, attach)
	if ExitCode(err) != 2 || !strings.Contains(msg, "Documents folder") {
		t.Fatalf("attach %q via MCP: exit %d, %s", attach, ExitCode(err), msg)
	}
}

func TestTBMCPAttachInsideDocumentsAccepted(t *testing.T) {
	f := tbSetupAttach(t, true)
	spec, msg, err := tbDraftAttach(t, f, f.inside+","+filepath.Join(f.docs, "sub", "..", "second.txt"))
	if err != nil {
		t.Fatal(msg)
	}
	if len(spec.Attachments) != 2 || !strings.EqualFold(spec.Attachments[0], f.inside) || !strings.EqualFold(spec.Attachments[1], filepath.Join(f.docs, "second.txt")) {
		t.Fatalf("attachments = %v", spec.Attachments)
	}
}

func TestTBMCPAttachOutsideDocumentsRejected(t *testing.T) {
	f := tbSetupAttach(t, true)
	tbAssertAttachRejected(t, f, f.outside)
	tbAssertAttachRejected(t, f, filepath.Join(filepath.Dir(f.docs), "DocumentsX", "x.txt"))
	tbAssertAttachRejected(t, f, f.inside+","+f.outside)
}

func TestTBMCPAttachBoundProfileSurfaceRestricted(t *testing.T) {
	f := tbSetupAttach(t, false)
	t.Setenv(mcpBoundProfileEnv, "default")
	if !tbMCPSurface() {
		t.Fatal("bound MCP profile not treated as MCP surface")
	}
	if _, err := tbResolveMCPAttachment(f.outside); err == nil {
		t.Fatal("outside file accepted")
	}
}

func TestTBMCPAttachTraversalRejected(t *testing.T) {
	f := tbSetupAttach(t, true)
	tbAssertAttachRejected(t, f, filepath.Join(f.docs, "..", "Outside", "secret.txt"))
}

func TestTBMCPAttachSymlinkOutsideRejected(t *testing.T) {
	f := tbSetupAttach(t, true)
	link := filepath.Join(f.docs, "link.txt")
	if err := os.Symlink(f.outside, link); err != nil {
		t.Skipf("symlink not permitted: %v", err)
	}
	tbAssertAttachRejected(t, f, link)
}

func TestTBMCPAttachJunctionOutsideRejected(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions are Windows-only")
	}
	f := tbSetupAttach(t, true)
	junction := filepath.Join(f.docs, "junction")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, filepath.Dir(f.outside)).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v %s", err, out)
	}
	tbAssertAttachRejected(t, f, filepath.Join(junction, "secret.txt"))
}

func TestTBTerminalAttachUnrestricted(t *testing.T) {
	f := tbSetupAttach(t, false)
	spec, msg, err := tbDraftAttach(t, f, f.outside)
	if err != nil {
		t.Fatal(msg)
	}
	if len(spec.Attachments) != 1 || spec.Attachments[0] != f.outside {
		t.Fatalf("attachments = %v", spec.Attachments)
	}
}
