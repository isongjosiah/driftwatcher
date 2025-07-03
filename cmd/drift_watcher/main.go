package main

import (
	"context"
	"drift-watcher/cmd"
)

func main() {
	ctx := context.Background()
	cmd.Execute(ctx)
}
