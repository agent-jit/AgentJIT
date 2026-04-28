package trace

import (
	"testing"
)

func TestTokenizeBashCommand_LiteralOnly(t *testing.T) {
	tokens := TokenizeBashCommand("ls -la")
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2", len(tokens))
	}
	if tokens[0].Value != "ls" || !tokens[0].Literal {
		t.Errorf("token[0] = %+v, want literal 'ls'", tokens[0])
	}
	if tokens[1].Value != "-la" || !tokens[1].Literal {
		t.Errorf("token[1] = %+v, want literal '-la'", tokens[1])
	}
}

func TestTokenizeBashCommand_PathBecomesVar(t *testing.T) {
	tokens := TokenizeBashCommand("cat /home/user/project/main.go")
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2", len(tokens))
	}
	if tokens[0].Value != "cat" || !tokens[0].Literal {
		t.Errorf("token[0] = %+v, want literal 'cat'", tokens[0])
	}
	if tokens[1].Literal {
		t.Errorf("token[1] should be variable (path), got literal")
	}
}

func TestTokenizeBashCommand_UUIDBecomesVar(t *testing.T) {
	tokens := TokenizeBashCommand("kubectl delete pod abc12345-def6-7890-abcd-ef1234567890")
	found := false
	for _, tok := range tokens {
		if !tok.Literal && tok.Value == "abc12345-def6-7890-abcd-ef1234567890" {
			found = true
		}
	}
	if !found {
		t.Error("UUID should be detected as variable token")
	}
}

func TestTokenizeBashCommand_NamespaceFlag(t *testing.T) {
	tokens := TokenizeBashCommand("kubectl get pods -n staging")
	for i, tok := range tokens {
		if tok.Value == "-n" && !tok.Literal {
			t.Errorf("flag -n should be literal")
		}
		if i > 0 && tokens[i-1].Value == "-n" && tok.Literal {
			t.Errorf("value after -n should be variable, got literal")
		}
	}
}

func TestTokenizeBashCommand_IPBecomesVar(t *testing.T) {
	tokens := TokenizeBashCommand("ping 192.168.1.100")
	found := false
	for _, tok := range tokens {
		if !tok.Literal && tok.Value == "192.168.1.100" {
			found = true
		}
	}
	if !found {
		t.Error("IP address should be detected as variable token")
	}
}

func TestTokenizeBashCommand_FlagEqualsValue(t *testing.T) {
	tokens := TokenizeBashCommand("kubectl get pods --namespace=staging")
	if len(tokens) != 5 {
		t.Fatalf("got %d tokens, want 5: %+v", len(tokens), tokens)
	}
	// --namespace=staging should be split into flag (literal) + value (variable)
	if tokens[3].Value != "--namespace" || !tokens[3].Literal {
		t.Errorf("token[3] = %+v, want literal '--namespace'", tokens[3])
	}
	if tokens[4].Value != "staging" || tokens[4].Literal {
		t.Errorf("token[4] = %+v, want variable 'staging'", tokens[4])
	}
}

func TestInputShapeBash(t *testing.T) {
	input := map[string]interface{}{
		"command": "kubectl get pods -n staging",
	}
	shape := InputShape("Bash", input)
	if shape["command"] != "kubectl get pods -n {VAR}" {
		t.Errorf("shape[command] = %q", shape["command"])
	}
}

func TestInputShapeBash_DifferentNamespace(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{"command": "kubectl get pods -n staging"})
	shape2 := InputShape("Bash", map[string]interface{}{"command": "kubectl get pods -n production"})
	if shape1["command"] != shape2["command"] {
		t.Errorf("same structure should produce same shape: %q vs %q", shape1["command"], shape2["command"])
	}
}

func TestInputShapeNonBash(t *testing.T) {
	input := map[string]interface{}{
		"file_path": "/some/path.go",
		"line":      42,
	}
	shape := InputShape("Read", input)
	if shape["file_path"] != "{STRING}" {
		t.Errorf("shape[file_path] = %q, want {STRING}", shape["file_path"])
	}
	if shape["line"] != "{NUMBER}" {
		t.Errorf("shape[line] = %q, want {NUMBER}", shape["line"])
	}
}

func TestNodeID_SameShapeSameID(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{"command": "kubectl get pods -n staging"})
	shape2 := InputShape("Bash", map[string]interface{}{"command": "kubectl get pods -n production"})
	id1 := NodeID("Bash", shape1)
	id2 := NodeID("Bash", shape2)
	if id1 != id2 {
		t.Errorf("same tool+shape should produce same NodeID: %d vs %d", id1, id2)
	}
}

func TestNodeID_DifferentToolDifferentID(t *testing.T) {
	shape := map[string]string{"command": "ls"}
	id1 := NodeID("Bash", shape)
	id2 := NodeID("Write", shape)
	if id1 == id2 {
		t.Error("different tools should produce different NodeIDs")
	}
}

func TestNormalizedBashShape_FlagOrderInvariance(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{
		"command": "az repos pr create --org myorg --project myproj --target-branch main",
	})
	shape2 := InputShape("Bash", map[string]interface{}{
		"command": "az repos pr create --project yourproj --org yourorg --target-branch dev",
	})
	if shape1["command"] != shape2["command"] {
		t.Errorf("flag order should not matter:\n  got1: %q\n  got2: %q", shape1["command"], shape2["command"])
	}
	want := "az repos pr create --org {VAR} --project {VAR} --target-branch {VAR}"
	if shape1["command"] != want {
		t.Errorf("shape = %q, want %q", shape1["command"], want)
	}
}

func TestNormalizedBashShape_DifferentFlagSets(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{
		"command": "az repos pr create --org myorg --project myproj",
	})
	shape2 := InputShape("Bash", map[string]interface{}{
		"command": "az repos pr create --org myorg --squash",
	})
	if shape1["command"] == shape2["command"] {
		t.Errorf("different flag sets should produce different shapes: %q", shape1["command"])
	}
}

func TestNormalizedBashShape_BooleanFlagsSorted(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{
		"command": "git commit -v --no-edit -m msg",
	})
	shape2 := InputShape("Bash", map[string]interface{}{
		"command": "git commit --no-edit -m msg2 -v",
	})
	if shape1["command"] != shape2["command"] {
		t.Errorf("boolean flags should sort equally:\n  got1: %q\n  got2: %q", shape1["command"], shape2["command"])
	}
}

func TestNormalizedBashShape_Heredoc(t *testing.T) {
	shape := InputShape("Bash", map[string]interface{}{
		"command": "cat <<EOF\nsome body text\nEOF",
	})
	if shape["command"] != "cat {HEREDOC}" {
		t.Errorf("heredoc shape = %q, want %q", shape["command"], "cat {HEREDOC}")
	}
}

func TestNormalizedBashShape_PositionalArgs(t *testing.T) {
	shape := InputShape("Bash", map[string]interface{}{
		"command": "git add /path/to/file1.go /path/to/file2.go",
	})
	if shape["command"] != "git add {VAR}" {
		t.Errorf("positional args shape = %q, want %q", shape["command"], "git add {VAR}")
	}
}

func TestNormalizedBashShape_FlagEqualsValueSorted(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{
		"command": "kubectl get pods --namespace=staging --output=json",
	})
	shape2 := InputShape("Bash", map[string]interface{}{
		"command": "kubectl get pods --output=yaml --namespace=production",
	})
	if shape1["command"] != shape2["command"] {
		t.Errorf("--flag=value order should not matter:\n  got1: %q\n  got2: %q", shape1["command"], shape2["command"])
	}
}

func TestNodeID_FlagOrderInvariance(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{
		"command": "az repos pr create --org X --project Y",
	})
	shape2 := InputShape("Bash", map[string]interface{}{
		"command": "az repos pr create --project Y --org X",
	})
	id1 := NodeID("Bash", shape1)
	id2 := NodeID("Bash", shape2)
	if id1 != id2 {
		t.Errorf("same command with reordered flags should produce same NodeID: %d vs %d", id1, id2)
	}
}

// --- shellFields tests ---

func TestShellFields_Simple(t *testing.T) {
	got := shellFields("az repos pr create")
	want := []string{"az", "repos", "pr", "create"}
	assertFields(t, got, want)
}

func TestShellFields_DoubleQuoted(t *testing.T) {
	got := shellFields(`--title "Fix something broken"`)
	want := []string{"--title", "Fix something broken"}
	assertFields(t, got, want)
}

func TestShellFields_SingleQuoted(t *testing.T) {
	got := shellFields(`--msg 'hello world'`)
	want := []string{"--msg", "hello world"}
	assertFields(t, got, want)
}

func TestShellFields_EmptyInput(t *testing.T) {
	got := shellFields("")
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func assertFields(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d fields %v, want %d fields %v", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("field[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// --- extractFirstPipeSegment tests ---

func TestExtractFirstPipeSegment_NoPipe(t *testing.T) {
	got := extractFirstPipeSegment("dotnet build")
	if got != "dotnet build" {
		t.Errorf("got %q, want %q", got, "dotnet build")
	}
}

func TestExtractFirstPipeSegment_SimplePipe(t *testing.T) {
	got := extractFirstPipeSegment("dotnet build 2>&1 | tail -20")
	if got != "dotnet build 2>&1" {
		t.Errorf("got %q, want %q", got, "dotnet build 2>&1")
	}
}

func TestExtractFirstPipeSegment_PipeInsideDoubleQuotes(t *testing.T) {
	got := extractFirstPipeSegment(`git commit -m "foo | bar"`)
	if got != `git commit -m "foo | bar"` {
		t.Errorf("got %q, want quoted pipe preserved", got)
	}
}

func TestExtractFirstPipeSegment_PipeInsideSingleQuotes(t *testing.T) {
	got := extractFirstPipeSegment(`grep -E 'error|warning' log.txt`)
	if got != `grep -E 'error|warning' log.txt` {
		t.Errorf("got %q, want quoted pipe preserved", got)
	}
}

func TestExtractFirstPipeSegment_PipeInsideSubstitution(t *testing.T) {
	got := extractFirstPipeSegment(`echo "$(cat file | head)" | wc -l`)
	if got != `echo "$(cat file | head)"` {
		t.Errorf("got %q, want substitution pipe preserved", got)
	}
}

// --- PreprocessBashCommand tests ---

func TestPreprocess_CdPrefix(t *testing.T) {
	got := PreprocessBashCommand("cd /path/to/repo && az repos pr create --org X")
	want := "az repos pr create --org X"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_ChainedCdPrefix(t *testing.T) {
	got := PreprocessBashCommand("cd foo && cd bar && cmd arg")
	want := "cmd arg"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_EnvVarPrefix(t *testing.T) {
	got := PreprocessBashCommand("PYTHONIOENCODING=utf-8 az rest --url X")
	want := "az rest --url X"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_QuotedEnvVarPrefix(t *testing.T) {
	got := PreprocessBashCommand(`REPO_ID="abc-123" az rest --url X`)
	want := "az rest --url X"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_MultipleEnvVars(t *testing.T) {
	got := PreprocessBashCommand("FOO=1 BAR=2 command")
	want := "command"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_TimeoutPrefix(t *testing.T) {
	got := PreprocessBashCommand("timeout 50 az repos pr policy list --id 1")
	want := "az repos pr policy list --id 1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_PipeChain(t *testing.T) {
	got := PreprocessBashCommand("dotnet build 2>&1 | tail -20")
	want := "dotnet build"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_StderrRedirect(t *testing.T) {
	got := PreprocessBashCommand("dotnet build 2>&1")
	want := "dotnet build"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_DevNullRedirect(t *testing.T) {
	got := PreprocessBashCommand("cmd 2>/dev/null")
	want := "cmd"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_CombinedAll(t *testing.T) {
	got := PreprocessBashCommand("cd /repo && PYTHONIOENCODING=utf-8 timeout 50 az rest 2>&1 | head")
	want := "az rest"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_NoChanges(t *testing.T) {
	got := PreprocessBashCommand("git status")
	want := "git status"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreprocess_PipeInsideQuotesPreserved(t *testing.T) {
	got := PreprocessBashCommand(`git commit -m "foo | bar"`)
	want := `git commit -m "foo | bar"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- Integration: end-to-end shape matching with preprocessing ---

func TestInputShape_CdPrefixNormalization(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{
		"command": "cd /repo && az repos pr create --org X",
	})
	shape2 := InputShape("Bash", map[string]interface{}{
		"command": "az repos pr create --org Y",
	})
	if shape1["command"] != shape2["command"] {
		t.Errorf("cd prefix should be stripped:\n  got1: %q\n  got2: %q", shape1["command"], shape2["command"])
	}
}

func TestInputShape_EnvVarPrefixNormalization(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{
		"command": "PYTHONIOENCODING=utf-8 az repos pr create --org X",
	})
	shape2 := InputShape("Bash", map[string]interface{}{
		"command": "az repos pr create --org Y",
	})
	if shape1["command"] != shape2["command"] {
		t.Errorf("env var prefix should be stripped:\n  got1: %q\n  got2: %q", shape1["command"], shape2["command"])
	}
}

func TestInputShape_PipeNormalization(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{
		"command": "dotnet build 2>&1 | tail -20",
	})
	shape2 := InputShape("Bash", map[string]interface{}{
		"command": "dotnet build 2>&1 | grep error",
	})
	if shape1["command"] != shape2["command"] {
		t.Errorf("pipe chain should be stripped:\n  got1: %q\n  got2: %q", shape1["command"], shape2["command"])
	}
}

func TestInputShape_QuotedFlagValues(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{
		"command": `az repos pr create --title "Fix service startup crash"`,
	})
	shape2 := InputShape("Bash", map[string]interface{}{
		"command": `az repos pr create --title "Fix other thing"`,
	})
	if shape1["command"] != shape2["command"] {
		t.Errorf("quoted flag values should match:\n  got1: %q\n  got2: %q", shape1["command"], shape2["command"])
	}
}

func TestInputShape_FullRealWorldExample(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{
		"command": `cd d:/Repos/workload-eventstream && az repos pr create --title "Fix service startup crash" --org https://dev.azure.com/powerbi --project MWC --target-branch main --source-branch fix/thing`,
	})
	shape2 := InputShape("Bash", map[string]interface{}{
		"command": `az repos pr create --title "Fix other thing" --org https://dev.azure.com/powerbi --project MWC --target-branch main --source-branch fix/other`,
	})
	if shape1["command"] != shape2["command"] {
		t.Errorf("real-world commands should match:\n  got1: %q\n  got2: %q", shape1["command"], shape2["command"])
	}
}
