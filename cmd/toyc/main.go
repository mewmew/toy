// toyc is a toy compiler written in Go.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mewkiz/pkg/term"
	"golang.org/x/tools/go/packages"
)

var (
	// dbg is a logger which logs debug messages with "toyc:" prefix to standard
	// error.
	dbg = log.New(os.Stderr, term.MagentaBold("toyc:")+" ", 0)
	// warn is a logger which logs warning messages with "toyc:" prefix to standard
	// error.
	warn = log.New(os.Stderr, term.RedBold("toyc:")+" ", 0)
)

func usage() {
	const use = `
Usage: toyc [OPTION]... [packages]
`
	fmt.Fprintln(os.Stderr, use[1:])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	// Pass command-line arguments uninterpreted to packages.Load so that it can
	// interpret them according to the conventions of the underlying build
	// system.
	cfg := &packages.Config{Mode: packages.LoadAllSyntax}
	pkgs, err := packages.Load(cfg, flag.Args()...)
	if err != nil {
		log.Fatalf("unable to load packages: %+v", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}
	// Compile packages.
	c := newCompiler()
	packages.Visit(pkgs, c.pre, c.post)
	switch len(c.errs) {
	case 0:
		// no error during compilation.
	case 1:
		log.Fatalf("error during compilation: %v", c.errs[0])
	default:
		buf := &bytes.Buffer{}
		fmt.Fprintf(buf, "%d errors during compilation:", len(c.errs))
		for _, err := range c.errs {
			fmt.Fprintf(buf, "\n\t%s", err)
		}
		log.Fatal(buf.String())
	}
	// Print compiled LLVM IR modules.
	for _, m := range c.modules {
		fmt.Println(m.String())
	}
}
