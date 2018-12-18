package lower

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"

	"github.com/llir/llvm/ir/types"
	"github.com/pkg/errors"
)

// === [ go/ast types API ] ====================================================

// resolveTypeDefs resolves the type definitions of the given Go package.
func (gen *Generator) resolveTypeDefs() {
	// Index type identifiers and create scaffolding IR type definitions (without
	// bodies).
	for _, file := range gen.pkg.Syntax {
		for _, decl := range file.Decls {
			if decl, ok := decl.(*ast.GenDecl); ok {
				if decl.Tok != token.TYPE {
					continue
				}
				for _, spec := range decl.Specs {
					ts := spec.(*ast.TypeSpec)
					typeName := ts.Name.String()
					gen.old.typeDefs[typeName] = ts.Type
				}
			}
		}
	}
	for typeName, oldType := range gen.old.typeDefs {
		t := gen.newASTType(typeName, oldType)
		t.SetName(typeName)
		gen.new.typeDefs[typeName] = t
	}
	// Translate AST type definitions to IR.
	for typeName, oldType := range gen.old.typeDefs {
		new := gen.new.typeDefs[typeName]
		gen.irASTTypeDef(new, oldType)
	}
}

// newASTType creates a new LLVM IR type (without body) based on the given Go type.
func (gen *Generator) newASTType(typeName string, old ast.Expr) types.Type {
	switch old := old.(type) {
	case *ast.Ident:
		newName := old.String()
		newType := gen.old.typeDefs[newName]
		return gen.newASTType(newName, newType)
	case *ast.StarExpr:
		return &types.PointerType{TypeName: typeName}
	case *ast.StructType:
		return &types.StructType{TypeName: typeName}
	default:
		panic(fmt.Errorf("support for type %T not yet implemented", old))
	}
}

// irASTTypeDef translates the AST type into an equivalent IR type. A new IR
// type correspoding to the AST type is created if t is nil, otherwise the body
// of t is populated. Named types are resolved through gen.new.typeDefs.
func (gen *Generator) irASTTypeDef(t types.Type, old ast.Expr) (types.Type, error) {
	switch old := old.(type) {
	case *ast.Ident:
		return gen.irASTNamedType(t, old)
	case *ast.StarExpr:
		return gen.irASTPointerType(t, old)
	case *ast.StructType:
		return gen.irASTStructType(t, old)
	default:
		panic(fmt.Errorf("support for type %T not yet implemented", old))
	}
}

// --- [ Pointer type ] --------------------------------------------------------

// irASTPointerType translates the AST pointer type into an equivalent IR type.
// A new IR type correspoding to the AST type is created if t is nil, otherwise
// the body of t is populated.
func (gen *Generator) irASTPointerType(t types.Type, old *ast.StarExpr) (types.Type, error) {
	typ, ok := t.(*types.PointerType)
	if t == nil {
		typ = &types.PointerType{}
	} else if !ok {
		panic(fmt.Errorf("invalid IR type for AST pointer type; expected *types.PointerType, got %T", t))
	}
	// Element type.
	elemType, err := gen.irASTType(old.X)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	typ.ElemType = elemType
	return typ, nil
}

// --- [ Named type ] ----------------------------------------------------------

// irASTNamedType translates the AST named type into an equivalent IR type.
func (gen *Generator) irASTNamedType(t types.Type, old *ast.Ident) (types.Type, error) {
	// TODO: make use of t?
	// Resolve named type.
	typeName := old.String()
	typ, ok := gen.new.typeDefs[typeName]
	if !ok {
		return nil, errors.Errorf("unable to locate type definition of named type %q", typeName)
	}
	return typ, nil
}

// --- [ Struct type ] ---------------------------------------------------------

// irASTStructType translates the AST struct type into an equivalent IR type. A
// new IR type correspoding to the AST type is created if t is nil, otherwise
// the body of t is populated.
func (gen *Generator) irASTStructType(t types.Type, old *ast.StructType) (types.Type, error) {
	typ, ok := t.(*types.StructType)
	if t == nil {
		typ = &types.StructType{}
	} else if !ok {
		panic(fmt.Errorf("invalid IR type for AST struct type; expected *types.StructType, got %T", t))
	}
	// Fields.
	fields := gen.irParams(old.Fields)
	for _, field := range fields {
		typ.Fields = append(typ.Fields, field.Typ)
	}
	return typ, nil
}

// ### [ Helpers ] #############################################################

// irASTType returns the IR type corresponding to the given AST type.
func (gen *Generator) irASTType(old ast.Expr) (types.Type, error) {
	return gen.irASTTypeDef(nil, old)
}

// === [ go/types API ] ========================================================

// irTypeOf returns the LLVM IR type of the given Go expression.
func (gen *Generator) irTypeOf(expr ast.Expr) (types.Type, error) {
	goType := gen.pkg.TypesInfo.TypeOf(expr)
	return gen.irType(goType)
}

// irType returns the IR type of the given Go expression.
func (gen *Generator) irType(goType gotypes.Type) (types.Type, error) {
	switch goType := goType.(type) {
	case *gotypes.Basic:
		return gen.irBasicType(goType), nil
	default:
		panic(fmt.Errorf("support for Go type %T not yet implemented", goType))
	}
}

// CPU word size in number of bits.
const cpuWordSize = 64

// irBasicType returns the IR type of the given Go basic type.
func (gen *Generator) irBasicType(goType *gotypes.Basic) types.Type {
	// predeclared types
	switch goType.Kind() {
	case gotypes.Bool:
		return types.I1
	case gotypes.Int, gotypes.Uint:
		return types.NewInt(cpuWordSize)
	case gotypes.Int8, gotypes.Uint8:
		return types.I8
	case gotypes.Int16, gotypes.Uint16:
		return types.I16
	case gotypes.Int32, gotypes.Uint32:
		return types.I32
	case gotypes.Int64, gotypes.Uint64:
		return types.I64
	case gotypes.Uintptr:
		return types.NewInt(cpuWordSize)
	case gotypes.Float32:
		return types.Float
	case gotypes.Float64:
		return types.Double
	case gotypes.Complex64:
		var (
			realType    = types.Float
			complexType = types.Float
		)
		return types.NewStruct(realType, complexType)
	case gotypes.Complex128:
		var (
			realType    = types.Double
			complexType = types.Double
		)
		return types.NewStruct(realType, complexType)
	case gotypes.String:
		var (
			dataType = types.NewPointer(types.I8)
			lenType  = types.I64
		)
		return types.NewStruct(dataType, lenType)
	case gotypes.UnsafePointer:
		return types.NewInt(cpuWordSize)
	// types for untyped values
	case gotypes.UntypedBool:
		return types.I1
	case gotypes.UntypedInt:
		untypedInt := types.NewInt(64)
		untypedInt.SetName("untyped_int")
		gen.new.typeDefs["untyped_int"] = untypedInt
		return untypedInt
	case gotypes.UntypedRune:
		untypedRune := types.NewInt(32)
		untypedRune.SetName("untyped_rune")
		gen.new.typeDefs["untyped_rune"] = untypedRune
		return untypedRune
	case gotypes.UntypedFloat:
		untypedFloat := &types.FloatType{Kind: types.FloatKindDouble}
		untypedFloat.SetName("untyped_float")
		gen.new.typeDefs["untyped_float"] = untypedFloat
		return untypedFloat
	case gotypes.UntypedComplex:
		untypedFloat := &types.FloatType{Kind: types.FloatKindDouble}
		untypedFloat.SetName("untyped_float")
		var (
			realType    = untypedFloat
			complexType = untypedFloat
		)
		untypedComplex := types.NewStruct(realType, complexType)
		untypedComplex.SetName("untyped_complex")
		gen.new.typeDefs["untyped_complex"] = untypedComplex
		return untypedComplex
	case gotypes.UntypedString:
		var (
			dataType = types.NewPointer(types.I8)
			lenType  = types.I64
		)
		untypedString := types.NewStruct(dataType, lenType)
		untypedString.SetName("untyped_string")
		gen.new.typeDefs["untyped_string"] = untypedString
		return untypedString
	case gotypes.UntypedNil:
		untypedNil := types.NewPointer(types.I8)
		untypedNil.SetName("untyped_nil")
		gen.new.typeDefs["untyped_nil"] = untypedNil
		return untypedNil
	default:
		panic(fmt.Errorf("support for basic type of kind %v not yet implemented", goType.Kind()))
	}
}