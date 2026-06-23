package main

import (
	"fmt"
	"os"

	"github.com/tyandl/homelink/pkg/flair"
)

// apiHost is the Flair API base URL.
const apiHost = "https://api.flair.co"

func main() {
	client, err := flair.NewClient(
		apiHost,
		func() string { return os.Getenv("flair_client_id") },
		func() string { return os.Getenv("flair_client_secret") },
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	structures, err := client.GetStructures()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	for _, s := range structures.Resources {
		fmt.Printf("%s\t%s\t%s\n", s.ID, s.Attributes.Name, s.Attributes.Mode)
	}
}
