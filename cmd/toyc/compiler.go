package main

import (
	"github.com/llir/llvm/ir"
	"github.com/mewspring/toy/lower"
	"golang.org/x/tools/go/packages"
)

// compiler tracks the state of the compiler, including any errors encountered
// during compilation.
type compiler struct {
	// Compiled LLVM IR modules.
	modules []*ir.Module
	// List of errors encountered during compilation.
	errs []error
}

// newCompiler returns a new compiler.
func newCompiler() *compiler {
	return &compiler{}
}

// pre is invoked in pre-order traversal of the import graph. The returned
// boolean value determines whether imports of pkg are visited.
func (c *compiler) pre(pkg *packages.Package) bool {
	dbg.Println("pre:", pkg.Name)
	return true
}

// post is invoked in post-order traversal of the import graph.
func (c *compiler) post(pkg *packages.Package) {
	dbg.Println("post:", pkg.Name)
	eh := func(err error) {
		c.errs = append(c.errs, err)
	}
	gen := lower.NewGenerator(eh, pkg)
	m := gen.Lower()
	c.modules = append(c.modules, m)
}
