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
	storageFile := flag.String("storage", "", "path to customer storage JSON; required")
	id := flag.String("job", "", "existing job ID to recover without resubmission")
	flag.Parse()
	client, err := flashdata.New(flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY"), DataOrigin: os.Getenv("FLASHDATA_DATA_ORIGIN"), ManagementOrigin: os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN")})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 31*time.Minute)
	defer cancel()
	if *id == "" {
		if *storageFile == "" {
			log.Fatal("pass -storage storage.json or recover with -job job_ID")
		}
		b, err := os.ReadFile(*storageFile)
		if err != nil {
			log.Fatal("could not read storage file")
		}
		var storage flashdata.YouTubeDownloadParamsStorage
		if err = json.Unmarshal(b, &storage); err != nil {
			log.Fatal("invalid storage JSON")
		}
		job, err := client.YouTube.Download(ctx, flashdata.YouTubeDownloadInput{VideoID: "dQw4w9WgXcQ", Params: flashdata.YouTubeDownloadParams{DownloadType: flashdata.Ptr("audio"), OutputFormat: flashdata.Ptr("mp3"), Storage: storage}}, "")
		if err != nil {
			log.Fatal(err)
		}
		*id = job.ID
		record, _ := json.Marshal(map[string]string{"id": job.ID})
		if err = os.WriteFile("download.job.json", record, 0600); err != nil {
			log.Fatalf("accepted job %s; recovery record could not be written", job.ID)
		}
		log.Printf("Accepted %s. Recover with -job %s", job.ID, job.ID)
	}
	if _, err = client.Jobs.Wait(ctx, *id, flashdata.PollOptions{Timeout: 30 * time.Minute}); err != nil {
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
