"""Build a local Go module proxy archive and verify a clean packaged consumer.

Usage: python scripts/verify_package.py --go /absolute/path/to/go --dependency-cache /path/to/module/cache
No publishing or GitHub access is performed. The local-only proxy includes the pinned
schema dependency copied from Go's verified download cache. GOSUMDB=off applies only
to the isolated local candidate verification; it is not a user installation setting.
"""

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import zipfile
from pathlib import Path

from release_manifest import load_release_files, write_zip_member

ROOT = Path(__file__).resolve().parents[1]
MODULE = "github.com/flashdatadev/flashdata-go"
VERSION = "v1.0.0"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", required=True)
    parser.add_argument("--dependency-cache", required=True)
    args = parser.parse_args()
    go = str(Path(args.go).resolve())
    base = ROOT / "dist/package-check"
    base.mkdir(parents=True, exist_ok=True)
    output = Path(tempfile.mkdtemp(prefix="verification-", dir=base))
    proxy = output / "proxy"
    version = proxy / MODULE / "@v"
    version.mkdir(parents=True, exist_ok=True)
    archive = version / (VERSION + ".zip")
    entries = load_release_files(ROOT)
    go_sources = {p.relative_to(ROOT).as_posix() for p in ROOT.glob("*.go")}
    go_sources.update(
        p.relative_to(ROOT).as_posix() for p in ROOT.glob("examples/*/*.go")
    )
    if not go_sources.issubset(entries):
        parser.error(
            "new Go source files must be reviewed and added to release-files.json"
        )
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as z:
        for name in entries:
            write_zip_member(
                z, f"{MODULE}@{VERSION}/{name}", (ROOT / name).read_bytes()
            )
    required = {
        "go.mod",
        "go.sum",
        "LICENSE",
        "README.md",
        "README.zh-CN.md",
        "client.go",
        "examples/bulk/main.go",
        "docs/onboarding.md",
        "internal/contract/schemas.json",
    }
    assert required.issubset(entries)
    (version / (VERSION + ".mod")).write_bytes((ROOT / "go.mod").read_bytes())
    (version / (VERSION + ".info")).write_text(
        json.dumps({"Version": VERSION, "Time": "2026-10-21T00:00:00Z"}) + "\n",
        encoding="utf-8",
    )
    (version / "list").write_text(VERSION + "\n", encoding="utf-8")
    dependency = "github.com/santhosh-tekuri/jsonschema/v5"
    source = Path(args.dependency_cache) / "cache/download" / dependency / "@v"
    dest = proxy / dependency / "@v"
    dest.mkdir(parents=True, exist_ok=True)
    for suffix in ["zip", "mod", "info"]:
        shutil.copyfile(source / ("v5.3.1." + suffix), dest / ("v5.3.1." + suffix))
    (dest / "list").write_text("v5.3.1\n")
    consumer = output / "consumer"
    consumer.mkdir(exist_ok=True)
    (consumer / "go.mod").write_text(
        f"module local.example/consumer\n\ngo 1.25.0\n\nrequire {MODULE} {VERSION}\n"
    )
    (consumer / "main.go").write_text(
        """package main
import ("context";"fmt";"io";"net/http";"strings";flashdata "github.com/flashdatadev/flashdata-go")
type transport struct{}
func (transport) RoundTrip(r *http.Request)(*http.Response,error){
 if r.Header.Get("Accept")!="text/plain" || r.Header.Get("X-Respond-With")!="markdown" {panic("missing format headers")}
 return &http.Response{StatusCode:200,Header:http.Header{"Content-Type":[]string{"text/plain; charset=utf-8"}},Body:io.NopCloser(strings.NewReader("# Offline Markdown"))},nil
}
func main(){
 c,err:=flashdata.New(flashdata.Options{APIKey:"fd_offline",HTTPClient:&http.Client{Transport:transport{}}});if err!=nil{panic(err)}
 response,err:=c.Google.SearchWithFormat(context.Background(),flashdata.GoogleSearchInput{Query:"offline"},flashdata.ContentOptions{Accept:flashdata.AcceptText,RespondWith:flashdata.RespondMarkdown});if err!=nil || response.Text!="# Offline Markdown"{panic("formatted consumer failed")}
 var model flashdata.MarkdownQueryResult
 response=&flashdata.FormattedResponse{JSON:[]byte(`{"source":"google_search","status":"completed","content":"# Offline Markdown","id":"job_example","completed_at":"2026-10-23T00:00:00Z"}`)}
 if err=response.DecodeJSON(&model);err!=nil || model.ID!="job_example" || model.CompletedAt==""{panic("Markdown model failed")}
 fmt.Println(flashdata.Version)
}
""",
        encoding="utf-8",
    )
    env = os.environ.copy()
    env.update(
        {
            "GOPROXY": proxy.as_uri(),
            "GOSUMDB": "off",
            "GONOSUMDB": MODULE,
            "GOMODCACHE": str(output / "module-cache"),
            "GOWORK": "off",
            "GOTOOLCHAIN": "local",
            "CGO_ENABLED": "0",
        }
    )
    env.pop("GOOS", None)
    env.pop("GOARCH", None)
    subprocess.run([go, "mod", "tidy"], cwd=consumer, env=env, check=True)
    subprocess.run([go, "vet", "./..."], cwd=consumer, env=env, check=True)
    result = subprocess.run(
        [go, "run", "."],
        cwd=consumer,
        env=env,
        check=True,
        capture_output=True,
        text=True,
    )
    assert result.stdout.strip() == "1.0.0", result.stdout
    cross_builds = []
    binaries = output / "binaries"
    binaries.mkdir()
    for system, arch in [
        ("linux", "amd64"),
        ("linux", "arm64"),
        ("darwin", "amd64"),
        ("darwin", "arm64"),
        ("windows", "amd64"),
    ]:
        target_env = {**env, "GOOS": system, "GOARCH": arch}
        subprocess.run(
            [
                go,
                "build",
                "-trimpath",
                "-buildvcs=false",
                "-mod=readonly",
                "-o",
                str(binaries / f"consumer-{system}-{arch}"),
                ".",
            ],
            cwd=consumer,
            env=target_env,
            check=True,
        )
        cross_builds.append(f"{system}/{arch}")
    report = {
        "module": MODULE,
        "version": VERSION,
        "archive": str(archive),
        "sha256": hashlib.sha256(archive.read_bytes()).hexdigest(),
        "files": len(entries),
        "cleanConsumer": "passed",
        "liveRequests": 0,
        "crossBuilds": cross_builds,
        "allowlisted": True,
    }
    (output / "report.json").write_text(
        json.dumps(report, indent=2) + "\n", encoding="utf-8"
    )
    (base / "report.json").write_text(
        json.dumps(report, indent=2) + "\n", encoding="utf-8"
    )
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
