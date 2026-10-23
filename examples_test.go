package flashdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Builds and executes the actual shipped programs and README code against
// loopback fixtures. No external network, key, cloud bucket or trial is used.
func TestRunnableExamplesAndReadmes(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess example validation; run full go test ./... before release")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	goExe := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goExe += ".exe"
	}
	b, err := os.ReadFile("testdata/source-examples.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]struct{ Response json.RawMessage }
	if err = json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	storedFixtures := readFormatFixtures(t, "stored-result-formats.json")
	var jobMu sync.Mutex
	jobSource := SourceYouTubeMetadata
	part := gzipBytes(t, "{\"index\":0,\"source\":\"youtube_download\",\"video_id\":\"dQw4w9WgXcQ\",\"status\":\"completed\",\"results\":[]}\n")
	var requests, creations, videoCreations atomic.Int32
	var failNextStart atomic.Bool
	var base string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/v1/queries/realtime" {
			var q Query
			_ = json.NewDecoder(r.Body).Decode(&q)
			f, ok := fixtures[string(q.Source)]
			if !ok {
				t.Error("unknown source", q.Source)
				w.WriteHeader(500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(f.Response)
			return
		}
		if r.URL.Path == "/v1/queries" {
			var q Query
			_ = json.NewDecoder(r.Body).Decode(&q)
			jobMu.Lock()
			jobSource = q.Source
			jobMu.Unlock()
			respond(w, map[string]any{"id": "job_example", "source": q.Source, "status": "pending"})
			return
		}
		if r.URL.Path == "/v1/queries/batch" {
			var body struct{ Queries []Query }
			_ = json.NewDecoder(r.Body).Decode(&body)
			jobs := []any{}
			for i, q := range body.Queries {
				jobs = append(jobs, map[string]any{"id": fmt.Sprintf("job_example%d", i), "source": q.Source, "status": "pending"})
			}
			respond(w, map[string]any{"queries": jobs})
			return
		}
		switch r.URL.Path {
		case "/v2/api-access":
			respond(w, map[string]any{"result_origin": base})
		case "/v2/catalog/sources":
			respond(w, map[string]any{"data": []any{map[string]any{"id": "google_search", "name": "Google Search"}}})
		case "/v2/api-access/balance":
			respond(w, map[string]any{"balanceCredits": 10, "availableCredits": 10, "unit": "Credits", "scope": "account"})
		case "/v2/agent-connections/capabilities":
			respond(w, map[string]any{"protocolVersion": 2, "available": false})
		case "/v2/bulk-jobs":
			creations.Add(1)
			var request struct {
				Options map[string]any `json:"options"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error("invalid bulk example request", err)
			}
			if request.Options["download_type"] == "audio_video" {
				videoCreations.Add(1)
				if request.Options["output_format"] != "mp4" || request.Options["video_quality"] != "<=720" {
					t.Error("video options were not forwarded")
				}
			}
			respond(w, map[string]any{"job": map[string]any{"id": "bulk_examples", "status": "awaiting_upload"}, "upload": map[string]any{"href": base + "/upload", "method": "PUT", "contentType": "text/plain; charset=utf-8", "expiresAt": time.Now().Add(time.Minute).Format(time.RFC3339)}})
		case "/upload":
			if r.Header.Get("X-API-Key") != "" {
				t.Error("example upload leaked key")
			}
			w.WriteHeader(200)
		case "/v2/bulk-jobs/bulk_examples", "/v2/bulk-jobs/bulk_examples/start":
			if strings.HasSuffix(r.URL.Path, "/start") && failNextStart.Swap(false) {
				w.WriteHeader(503)
				respond(w, map[string]any{"code": "offline_failure", "detail": "Inspect the accepted job before trying again."})
				return
			}
			respond(w, map[string]any{"job": map[string]any{"id": "bulk_examples", "status": "completed"}})
		case "/v2/bulk-jobs/bulk_examples/results":
			respond(w, map[string]any{"id": "bulk_examples", "parts": []any{map[string]any{"number": 0}}})
		case "/v2/bulk-jobs/bulk_examples/results/0":
			_, _ = w.Write(part)
		case "/v2/jobs/job_example":
			jobMu.Lock()
			source := jobSource
			jobMu.Unlock()
			respond(w, map[string]any{"job": map[string]any{"id": "job_example", "source": source, "status": "completed"}})
		case "/v2/jobs/job_example/results":
			jobMu.Lock()
			source := jobSource
			jobMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(strings.ReplaceAll(string(storedFixtures[source][0].Response), "job_fixture0001", "job_example")))
		default:
			t.Error("example requested unknown route", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	base = server.URL
	env := []string{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(e), "FLASHDATA_") {
			env = append(env, e)
		}
	}
	env = append(env, "FLASHDATA_API_KEY="+testKey, "FLASHDATA_DATA_ORIGIN="+base, "FLASHDATA_MANAGEMENT_ORIGIN="+base, "GOWORK=off")
	run := func(t *testing.T, source string, args ...string) {
		t.Helper()
		dir := t.TempDir()
		binary := filepath.Join(dir, "example")
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		build := exec.CommandContext(ctx, goExe, "build", "-o", binary, source)
		build.Dir = root
		build.Env = env
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("example build failed: %v\n%s", err, out)
		}
		if err = os.WriteFile(filepath.Join(dir, "ids.txt"), []byte("dQw4w9WgXcQ\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "storage.json"), []byte(`{"storage_type":"s3","storage_url":"offline-bucket","access_key":"offline","secret_key":"offline-secret"}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "video-options.json"), []byte(`{"download_type":"audio_video","video_quality":"<=720","output_format":"mp4"}`), 0600); err != nil {
			t.Fatal(err)
		}
		execute := func(arguments ...string) ([]byte, error) {
			command := exec.CommandContext(ctx, binary, arguments...)
			command.Dir = dir
			command.Env = env
			return command.CombinedOutput()
		}
		if out, err := execute(args...); err != nil {
			t.Fatalf("example execution failed: %v\n%s", err, out)
		}
		if source == "./examples/background" || source == "./examples/bulk" {
			recordName := "background.job.json"
			if source == "./examples/bulk" {
				recordName = "receipts.jsonl.job.json"
			}
			recordPath := filepath.Join(dir, recordName)
			original, err := os.ReadFile(recordPath)
			if err != nil || len(original) == 0 {
				t.Fatal("example did not persist the accepted ID", err)
			}
			before := requests.Load()
			if out, err := execute(args...); err == nil {
				t.Fatalf("example overwrote existing state: %s", out)
			}
			after, err := os.ReadFile(recordPath)
			if err != nil || string(after) != string(original) || requests.Load() != before {
				t.Fatal("rerun changed the recovery record or contacted the API")
			}
			if source == "./examples/bulk" {
				// An orphaned record must also block submission when its output
				// file is absent; a separate Stat followed by WriteFile is unsafe.
				if err := os.WriteFile(filepath.Join(dir, "blocked.jsonl.job.json"), original, 0600); err != nil {
					t.Fatal(err)
				}
				blockedArgs := []string{"-ids", "ids.txt", "-out", "blocked.jsonl"}
				if out, err := execute(blockedArgs...); err == nil || requests.Load() != before {
					t.Fatalf("orphaned recovery record did not prevent API access: %v %s", err, out)
				}
				unchanged, err := os.ReadFile(filepath.Join(dir, "blocked.jsonl.job.json"))
				if err != nil || string(unchanged) != string(original) {
					t.Fatal("orphaned recovery record was overwritten")
				}
				createdBefore := creations.Load()
				failNextStart.Store(true)
				if out, err := execute("-ids", "ids.txt", "-out", "interrupted.jsonl"); err == nil {
					t.Fatalf("injected start failure was hidden: %s", out)
				}
				var record struct{ ID string }
				b, err := os.ReadFile(filepath.Join(dir, "interrupted.jsonl.job.json"))
				if err != nil || json.Unmarshal(b, &record) != nil || record.ID != "bulk_examples" || creations.Load() != createdBefore+1 {
					t.Fatal("accepted job was lost or creation retried")
				}
				if out, err := execute("-job", record.ID, "-out", "recovered.jsonl"); err != nil || creations.Load() != createdBefore+1 {
					t.Fatalf("recovery did not use the accepted job: %v %s", err, out)
				}
			}
		}
	}
	dirs, err := os.ReadDir("examples")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		t.Run(d.Name(), func(t *testing.T) {
			args := []string{}
			switch d.Name() {
			case "youtube_download":
				args = []string{"-storage", "storage.json"}
			case "bulk":
				args = []string{"-source", "youtube_download", "-ids", "ids.txt", "-storage", "storage.json", "-out", "receipts.jsonl"}
			}
			run(t, "./examples/"+d.Name(), args...)
		})
	}
	t.Run("bulk-video-options", func(t *testing.T) {
		before := videoCreations.Load()
		run(t, "./examples/bulk", "-source", "youtube_download", "-ids", "ids.txt", "-storage", "storage.json", "-options", "video-options.json", "-out", "receipts.jsonl")
		if videoCreations.Load() != before+1 {
			t.Fatal("MP4 example did not create exactly one video download task")
		}
	})
	re := regexp.MustCompile("(?s)```go\\r?\\n(.*?)```")
	var english []string
	var quickstartPrefix string
	for _, file := range []string{"README.md", "README.zh-CN.md"} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		blocks := re.FindAllStringSubmatch(string(content), -1)
		completePrograms := 0
		for i, block := range blocks {
			code := strings.TrimSpace(block[1])
			if file == "README.md" {
				english = append(english, code)
			} else if i >= len(english) || english[i] != code {
				t.Fatal("translated code drifted")
			}
			if strings.HasPrefix(code, "package main") {
				completePrograms++
				if i == 0 {
					prefix, _, found := strings.Cut(code, "\n\tresult, err := ")
					if !found {
						t.Fatal("quickstart needs a replaceable query section")
					}
					quickstartPrefix = prefix
				}
			} else {
				if quickstartPrefix == "" {
					t.Fatal("README snippets must follow the complete quickstart")
				}
				// Exercise exactly the replacement workflow described to readers:
				// keep the quickstart's imports/client/context, replace the query.
				code = quickstartPrefix + "\n" + code + "\n}\n"
			}
			// Customer examples only configure their API key. Inject test origins
			// here, requiring an exact match so the subprocess cannot call live APIs.
			const options = `flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY")}`
			if strings.Count(code, options) != 1 {
				t.Fatal("README program must have one explicit API-key client")
			}
			code = strings.Replace(code, options, `flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY"), DataOrigin: os.Getenv("FLASHDATA_DATA_ORIGIN"), ManagementOrigin: os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN")}`, 1)
			t.Run(fmt.Sprintf("%s/program-%d", file, i+1), func(t *testing.T) {
				dir := t.TempDir()
				source := filepath.Join(dir, "main.go")
				if err = os.WriteFile(source, []byte(code), 0600); err != nil {
					t.Fatal(err)
				}
				run(t, source)
			})
		}
		if completePrograms != 2 || len(blocks) < 3 {
			t.Fatal("README needs the quickstart, task snippets and complete bulk program")
		}
		if file == "README.zh-CN.md" && len(blocks) != len(english) {
			t.Fatal("translated code block count differs")
		}
	}
}

func TestMarkdownLinksAndExplicitCredentials(t *testing.T) {
	link := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".cache" || entry.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(b)
		text = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(text, "")
		text = regexp.MustCompile("`[^`]*`").ReplaceAllString(text, "")
		if !strings.Contains(text, "https://flashdata.dev") {
			t.Errorf("missing official website: %s", path)
		}
		for _, m := range link.FindAllStringSubmatch(text, -1) {
			target := strings.Split(m[1], "#")[0]
			if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if _, err = os.Stat(filepath.Join(filepath.Dir(path), filepath.FromSlash(target))); err != nil {
				t.Errorf("broken link in %s: %s", path, target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := os.ReadDir("examples")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join("examples", d.Name(), "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `APIKey: os.Getenv("FLASHDATA_API_KEY")`) {
			t.Errorf("missing explicit key in %s", d.Name())
		}
	}
}
