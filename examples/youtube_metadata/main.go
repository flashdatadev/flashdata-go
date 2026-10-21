package main

import (
	"context"
	"encoding/json"
	flashdata "github.com/flashdatadev/flashdata-go"
	"log"
	"os"
	"time"
)

func main() {
	client, err := flashdata.New(flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY"), DataOrigin: os.Getenv("FLASHDATA_DATA_ORIGIN"), ManagementOrigin: os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN")})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := client.YouTube.Metadata(ctx, flashdata.YouTubeMetadataInput{VideoID: "dQw4w9WgXcQ"})
	if err != nil {
		log.Fatal(err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
		log.Fatal(err)
	}
}
