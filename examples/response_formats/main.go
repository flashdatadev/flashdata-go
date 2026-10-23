// Select response headers for a single request. Each realtime/submit/batch run
// creates new work. Status/result reads use an already accepted job ID.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	flashdata "github.com/flashdatadev/flashdata-go"
)

func main() {
	operation := flag.String("operation", "realtime", "realtime, submit, batch, status or result")
	input := flag.String("input", `{"source":"google_search","query":"vector databases"}`, "JSON query; batch also accepts an array of queries")
	id := flag.String("job", "", "accepted job ID for status/result reads")
	accept := flag.String("accept", "application/json", "application/json or text/plain")
	representation := flag.String("respond-with", "", "markdown; applies only to realtime/result")
	flag.Parse()
	content := flashdata.ContentOptions{Accept: flashdata.Accept(*accept), RespondWith: flashdata.Representation(*representation)}
	envelope := flashdata.ResponseOptions{Accept: content.Accept}
	if *representation != "" && *operation != "realtime" && *operation != "result" {
		log.Fatal("-respond-with applies only to realtime or result")
	}
	client, err := flashdata.New(flashdata.Options{
		APIKey: os.Getenv("FLASHDATA_API_KEY"), DataOrigin: os.Getenv("FLASHDATA_DATA_ORIGIN"),
		ManagementOrigin: os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN"),
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var q flashdata.Query
	var queries []flashdata.Query
	if *operation == "realtime" || *operation == "submit" || *operation == "batch" {
		decoder := json.NewDecoder(strings.NewReader(*input))
		decoder.DisallowUnknownFields()
		if *operation == "batch" && strings.HasPrefix(strings.TrimSpace(*input), "[") {
			err = decoder.Decode(&queries)
		} else {
			err = decoder.Decode(&q)
			queries = []flashdata.Query{q}
		}
		if err != nil {
			log.Fatal("invalid query JSON: ", err)
		}
		var extra any
		if err = decoder.Decode(&extra); err != io.EOF {
			log.Fatal("-input must contain one JSON value")
		}
	}
	var response *flashdata.FormattedResponse
	switch *operation {
	case "realtime":
		response, err = client.Queries.RealtimeWithFormat(ctx, q, content)
	case "submit":
		response, err = client.Queries.SubmitWithFormat(ctx, q, envelope)
	case "batch":
		response, err = client.Queries.BatchWithFormat(ctx, queries, envelope)
	case "status":
		response, err = client.Jobs.GetWithFormat(ctx, *id, envelope)
	case "result":
		response, err = client.Jobs.ResultWithFormat(ctx, *id, content)
	default:
		log.Fatal("choose realtime, submit, batch, status or result")
	}
	if err != nil {
		log.Fatal(err)
	}
	if response.JSON != nil {
		_, err = fmt.Fprintln(os.Stdout, string(response.JSON))
	} else {
		_, err = fmt.Fprint(os.Stdout, response.Text)
	}
	if err != nil {
		log.Fatal(err)
	}
}
