// Example bulk runs a complete metadata or customer-storage download task.
// It intentionally has no automatic retries. Applications needing a durable
// signed-upload journal should use an application-owned secure store or the CLI.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	flashdata "github.com/flashdatadev/flashdata-go"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	ids := flag.String("ids", "", "UTF-8 file containing one YouTube ID per line")
	storage := flag.String("storage", "", "storage JSON; required for downloads")
	optionsFile := flag.String("options", "", "optional source options JSON, for example MP4 download settings")
	source := flag.String("source", "youtube_metadata", "one of the five YouTube bulk sources")
	id := flag.String("job", "", "existing bulk ID; never creates a replacement job")
	start := flag.Bool("start", false, "start an existing awaiting_upload job after its input was uploaded")
	resume := flag.Bool("resume", false, "resume an existing paused job after fixing its pause reason")
	out := flag.String("out", "results.jsonl", "output NDJSON; recovery ID saved to OUT.job.json")
	flag.Parse()
	client, err := flashdata.New(flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY"), DataOrigin: os.Getenv("FLASHDATA_DATA_ORIGIN"), ManagementOrigin: os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN")})
	if err != nil {
		return err
	}
	if *out == "" || (*start && *resume) {
		return fmt.Errorf("provide -out and choose at most one of -start/-resume")
	}
	// Check output writability before any paid work or job creation. Refuse to
	// overwrite a previous export; choose a fresh output name for recovery.
	f, err := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("output must be a new writable file")
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Minute)
	defer cancel()
	if *id == "" {
		if *start || *resume {
			return fmt.Errorf("-start/-resume require an existing -job")
		}
		input, err := os.Open(*ids)
		if err != nil {
			return fmt.Errorf("provide a readable -ids file")
		}
		defer input.Close()
		info, err := input.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 2<<30 {
			return fmt.Errorf("IDs file must be a regular file of 1 byte through 2 GiB")
		}
		// Upload the exact bytes validated here, even if the source file changes.
		snapshot, err := os.CreateTemp("", "flashdata-go-ids-*")
		if err != nil {
			return fmt.Errorf("could not create input snapshot")
		}
		defer os.Remove(snapshot.Name())
		defer snapshot.Close()
		scanner := bufio.NewScanner(io.TeeReader(io.LimitReader(input, (2<<30)+1), snapshot))
		pattern := regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
		count := 0
		for scanner.Scan() {
			if !pattern.MatchString(strings.TrimSpace(scanner.Text())) {
				return fmt.Errorf("IDs file must contain one 11-character video ID per line")
			}
			count++
		}
		if scanner.Err() != nil || count == 0 {
			return fmt.Errorf("could not validate IDs file")
		}
		snapshotInfo, err := snapshot.Stat()
		if err != nil || snapshotInfo.Size() > 2<<30 {
			return fmt.Errorf("input snapshot exceeds 2 GiB or could not be inspected")
		}
		if _, err = snapshot.Seek(0, 0); err != nil {
			return fmt.Errorf("could not rewind input")
		}
		options := map[string]any{}
		if *optionsFile != "" {
			b, err := os.ReadFile(*optionsFile)
			if err != nil {
				return fmt.Errorf("could not read options JSON")
			}
			if json.Unmarshal(b, &options) != nil || options == nil {
				return fmt.Errorf("options JSON must be an object")
			}
		}
		if *storage != "" {
			b, err := os.ReadFile(*storage)
			if err != nil {
				return fmt.Errorf("could not read storage configuration")
			}
			var dest map[string]any
			if json.Unmarshal(b, &dest) != nil || dest == nil {
				return fmt.Errorf("invalid storage JSON")
			}
			field := "result_storage"
			if *source == "youtube_download" {
				field = "storage"
			}
			options[field] = dest
		}
		if *source == "youtube_download" {
			if options["storage"] == nil {
				return fmt.Errorf("downloads require -storage")
			}
			if *optionsFile == "" {
				options["download_type"] = "audio"
				options["output_format"] = "mp3"
			}
		}
		request := flashdata.CreateBulkJobRequest{Source: flashdata.Source(*source), Options: options}
		if err = flashdata.ValidateBulkCreate(request); err != nil {
			return err
		}
		// Reserve atomically before the server accepts work. Never replace an
		// existing recovery record, including one left by an uncertain request.
		record, err := os.OpenFile(*out+".job.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("choose an output name without an existing recovery record")
		}
		defer record.Close()
		created, err := client.Bulk.Create(ctx, request)
		if err != nil {
			return err
		}
		*id = created.Job.ID
		// Save only the accepted ID, never the signed upload URL or credentials.
		if err = json.NewEncoder(record).Encode(map[string]string{"id": *id}); err == nil {
			err = record.Sync()
		}
		if err != nil {
			return fmt.Errorf("accepted %s; could not save recovery record", *id)
		}
		log.Printf("Accepted %s; recovery record %s.job.json", *id, *out)
		if err = client.Bulk.Upload(ctx, created, snapshot, snapshotInfo.Size()); err != nil {
			return fmt.Errorf("%s input upload failed: %w", *id, err)
		}
		if _, err = client.Bulk.Start(ctx, *id); err != nil {
			return err
		}
	} else {
		job, err := client.Bulk.Get(ctx, *id)
		if err != nil {
			return err
		}
		if job.Status == "awaiting_upload" {
			if !*start {
				return fmt.Errorf("%s awaits upload; supply its original signed upload then use -start", *id)
			}
			if _, err = client.Bulk.Start(ctx, *id); err != nil {
				return err
			}
		}
		if job.Status == "paused" {
			if !*resume {
				return fmt.Errorf("%s is paused; fix its cause with Bulk.Update then use -resume", *id)
			}
			if _, err = client.Bulk.Resume(ctx, *id); err != nil {
				return err
			}
		}
	}
	if _, err = client.Bulk.Wait(ctx, *id, flashdata.PollOptions{Timeout: time.Hour}); err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	failed := 0
	for row, err := range client.Bulk.Rows(ctx, *id) {
		if err != nil {
			return fmt.Errorf("partial output retained for %s: %w", *id, err)
		}
		if err = encoder.Encode(row); err != nil {
			return fmt.Errorf("could not write result output for %s", *id)
		}
		if string(row["status"]) != "\"completed\"" {
			failed++
		}
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("could not flush result output")
	}
	if failed > 0 {
		return fmt.Errorf("%d failed/invalid item receipts saved in %s; inspect before retrying", failed, *out)
	}
	log.Printf("Saved %s receipts to %s", *id, *out)
	return nil
}
