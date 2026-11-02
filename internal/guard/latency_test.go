package guard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// typicalCall is a Claude Code Bash call that the default policy allows, the most common hook call.
var typicalCall = corpusCase{
	name:  "typical",
	stdin: `{"session_id":"s1","transcript_path":"/tmp/t.jsonl","cwd":"/repo","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status && flutter test test/a_test.dart > out.txt 2>&1","description":"Run the tests"}}`,
	args:  []string{"{policies}/default.json"},
}

// knowledgeCalls are typical calls under a policy with a knowledge clone (K2): the same Bash call, and a Write,
// whose path the guard resolves to its real path.
var knowledgeCalls = []corpusCase{
	{name: "typical-knowledge", stdin: typicalCall.stdin, args: []string{"{policies}/knowledge.json"}},
	{name: "write-knowledge", args: []string{"{policies}/knowledge.json"},
		stdin: `{"session_id":"s1","cwd":"{project}","hook_event_name":"PreToolUse","tool_name":"Write","tool_input":{"file_path":"lib/feature/a.dart","content":"void a() {}"}}`},
}

// latencies runs the typical call n times through argv as a process, sorted.
func latencies(t testing.TB, f corpusFolders, n int, argv ...string) []time.Duration {
	t.Helper()
	return callLatencies(t, f, typicalCall, n, argv...)
}

// callLatencies runs call n times through argv as a process, sorted; the call must be allowed silently.
func callLatencies(t testing.TB, f corpusFolders, call corpusCase, n int, argv ...string) []time.Duration {
	t.Helper()
	out := make([]time.Duration, n)
	for i := range out {
		start := time.Now()
		result, err := runProcess(f, call, argv...)
		out[i] = time.Since(start)
		if err != nil || result != (outcome{}) {
			t.Fatalf("%v: %s, %v", argv, result, err)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	return sorted[int(float64(len(sorted)-1)*p)]
}

// TestGuardLatency times 1,000 `dashcrew guard` processes for one typical call and requires a median under
// 10 ms. Set DASHCREW_GUARD_LATENCY=1 to run it.
func TestGuardLatency(t *testing.T) {
	if os.Getenv("DASHCREW_GUARD_LATENCY") == "" {
		t.Skip("set DASHCREW_GUARD_LATENCY=1 to time 1,000 guard processes")
	}
	const calls = 1000
	f := newCorpusFolders(t)
	binary := buildDashcrew(t)
	goTimes := latencies(t, f, calls, binary, "guard")
	t.Logf("dashcrew guard: %d calls, median %v, p95 %v", calls, percentile(goTimes, 0.5), percentile(goTimes, 0.95))
	if median := percentile(goTimes, 0.5); median >= 10*time.Millisecond {
		t.Errorf("median %v, want under 10ms", median)
	}
	policy := strings.TrimSuffix(corpusPolicies["default"], "}") + `,"knowledge_clone":"~/.dashcrew/knowledge/notes"}`
	if err := os.WriteFile(filepath.Join(f.policies, "knowledge.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, call := range knowledgeCalls {
		times := callLatencies(t, f, call, calls, binary, "guard")
		t.Logf("dashcrew guard, %s: %d calls, median %v, p95 %v", call.name, calls, percentile(times, 0.5), percentile(times, 0.95))
		if median := percentile(times, 0.5); median >= 10*time.Millisecond {
			t.Errorf("%s: median %v, want under 10ms", call.name, median)
		}
	}
}

// BenchmarkGuardProcess is one typical hook call through a `dashcrew guard` process.
func BenchmarkGuardProcess(b *testing.B) {
	f := newCorpusFolders(b)
	binary := buildDashcrew(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if result, err := runProcess(f, typicalCall, binary, "guard"); err != nil || result != (outcome{}) {
			b.Fatalf("%s, %v", result, err)
		}
	}
}
