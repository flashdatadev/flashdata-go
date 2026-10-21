package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	flashdata "github.com/flashdatadev/flashdata-go"
)

func main() {
	client, err := flashdata.New(flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY"), DataOrigin: os.Getenv("FLASHDATA_DATA_ORIGIN"), ManagementOrigin: os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN")})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	for page, err := range client.Google.Pages(ctx, flashdata.GoogleSearchInput{Query: "Go tutorials", Params: &flashdata.GoogleSearchParams{Num: flashdata.Ptr(int64(10))}}, 2) {
		if err != nil {
			log.Fatal(err)
		}
		if err = json.NewEncoder(os.Stdout).Encode(page); err != nil {
			log.Fatal(err)
		}
	}
}
