package main

import (
	"fmt"
	"io"
	"os"
)

const serviceName = "sprout-api"

func main() {
	os.Exit(run(os.Stdout))
}

func run(stdout io.Writer) int {
	fmt.Fprintf(stdout, "%s: backend foundation ready\n", serviceName)
	return 0
}
