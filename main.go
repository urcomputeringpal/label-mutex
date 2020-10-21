package main

import (
	"fmt"

	"github.com/sethvargo/go-githubactions"
)

func main() {
	label := githubactions.GetInput("label")
	if label == "" {
		githubactions.Fatalf("missing input 'label'")
	}
	fmt.Println(label)
}
