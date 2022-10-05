package main

import (
	"fmt"
	"makeotel/parser"
	"os"
)

func main() {
	err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("this program takes one argument: path")
	}

	file := args[0]

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	p := parser.NewCallgrindParser(f)
	profile, err := p.Parse()
	if err != nil {
		return err
	}

	start := profile.Roots()[0]
	fmt.Println(start.Name)

	printCalls(profile, "", start)
	// for _, call := range start.Calls() {
	// 	fmt.Println("=> " + call.Callee_id)
	// 	fn, found := profile.GetFunction(call.Callee_id)
	// 	if found {

	// 	}
	// }
	// for _, fn := range profile.Roots() {
	// 	fmt.Println(fn.Name)

	// }

	return nil
}

func printCalls(profile *parser.Profile, indent string, fn *parser.Function) {
	// fmt.Println(indent + fn.Name + ":")

	for _, call := range fn.Calls() {
		fmt.Println(indent + "=> " + call.CalleeId)
		fn, found := profile.GetFunction(call.CalleeId)
		if found {
			printCalls(profile, indent+"  ", fn)
		}
	}
}
