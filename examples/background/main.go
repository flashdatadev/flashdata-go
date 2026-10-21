package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	flashdata "github.com/flashdatadev/flashdata-go"
)

func main() {
	id := flag.String("job", "", "existing job ID; omission submits one metadata query")
	record := flag.String("record", "background.job.json", "new recovery file; never overwrites an existing file")
	flag.Parse()
	client, err := flashdata.New(flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY"), DataOrigin: os.Getenv("FLASHDATA_DATA_ORIGIN"), ManagementOrigin: os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN")})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	if *id == "" {
		// Reserve the recovery path before submitting any work. An existing
		// record means recover its job, or explicitly choose a new record path.
		file, err := os.OpenFile(*record, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatal("choose a new -record path or recover with -job")
		}
		defer file.Close()
		job, err := client.YouTube.MetadataAsync(ctx, flashdata.YouTubeMetadataInput{VideoID: "dQw4w9WgXcQ"}, "")
		if err != nil {
			log.Fatal(err)
		}
		*id = job.ID
		// Persist the accepted ID before polling. No API key or storage secrets.
		if err = json.NewEncoder(file).Encode(map[string]string{"id": job.ID}); err == nil {
			err = file.Sync()
		}
		if err != nil {
			log.Fatalf("accepted job %s; could not save recovery file", job.ID)
		}
		log.Printf("Accepted %s. Recover with -job %s", job.ID, job.ID)
	}
	if _, err = client.Jobs.Wait(ctx, *id, flashdata.PollOptions{Timeout: 5 * time.Minute}); err != nil {
		log.Fatal(err)
	}
	result, err := client.Jobs.Result(ctx, *id)
	if err != nil {
		log.Fatal(err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
		log.Fatal(err)
	}
}
