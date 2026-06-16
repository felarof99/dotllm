// Command dotllm mirrors a project's .llm/ scratch dir into a central
// home archive (~/.llm/<repo>/<yyyy-mm-dd>[_<task>]/) via a symlink.
package main

import "github.com/felarof01/dotllm/cmd"

func main() {
	cmd.Execute()
}
