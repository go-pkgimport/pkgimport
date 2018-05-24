package gotype

import (
	"go/ast"
	"go/token"
	"strconv"
)

type astParser struct {
	importer *Importer
	method   map[string]Types // 方法
	nameds   Types            // 变量 函数 类型 导入的包
	src      string
}

// NewParser
func newParser(i *Importer, src string) *astParser {
	r := &astParser{
		importer: i,
		method:   map[string]Types{},
		nameds:   Types{},
		src:      src,
	}
	return r
}

// ParserFile 解析文件
func (r *astParser) ParserFile(src *ast.File) {
	// 解析全部顶级关键字
	for _, decl := range src.Decls {
		r.ParserDecl(decl)
	}
}

func (r *astParser) ParserPackage(pkg *ast.Package) {
	for _, file := range pkg.Files {
		r.ParserFile(file)
	}
}

// ParserDecl 解析顶级关键字
func (r *astParser) ParserDecl(decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		f := r.EvalType(d.Type)
		if d.Recv != nil {
			name, ok := typeName(d.Recv.List[0].Type)
			if ok { // 不是当前包的方法
				return
			}

			t := newTypeNamed(d.Name.Name, f, r)
			b := r.method[name]
			b.Add(t)
			r.method[name] = b
			return
		}

		t := newTypeNamed(d.Name.Name, f, r)
		r.nameds.Add(t)
		return
	case *ast.GenDecl:
		switch d.Tok {
		case token.CONST, token.VAR:
			r.ParserValue(d)
		case token.IMPORT:
			for _, spec := range d.Specs {
				s, ok := spec.(*ast.ImportSpec)
				if !ok {
					continue
				}
				path, err := strconv.Unquote(s.Path.Value)
				if err != nil {
					continue
				}

				if r.importer == nil {
					continue
				}

				if s.Name == nil {
					p := newTypeImport("", path, r.src, r.importer)
					r.nameds.AddNoRepeat(p)
				} else {
					switch s.Name.Name {
					case "_":
					case ".":
						p := r.importer.impor(path, r.src)
						if p == nil {
							continue
						}
						l := p.NumChild()
						for i := 0; i != l; i++ {
							r.nameds.AddNoRepeat(p.Child(i))
						}
					default:
						t := newTypeImport(s.Name.Name, path, r.src, r.importer)
						r.nameds.AddNoRepeat(t)
					}
				}
			}
		case token.TYPE:
			for _, spec := range d.Specs {
				s, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				tt := r.EvalType(s.Type)
				if s.Assign == 0 && tt.Kind() != Interface {
					tt = newTypeNamed(s.Name.Name, tt, r)
				} else {
					tt = newTypeAlias(s.Name.Name, tt)
				}
				r.nameds.Add(tt)
			}
		}
	}
}

// ParserValue 解析
func (r *astParser) ParserValue(decl *ast.GenDecl) {
	var prev, val Type
loop:
	for _, spec := range decl.Specs {
		prev = val
		s, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		val = nil
		if s.Type != nil { // 有类型声明
			val = r.EvalType(s.Type)
		} else {
			switch l := len(s.Values); l {
			case 0:
				if decl.Tok == token.CONST {
					val = prev
				}
			case 1:
				val = r.EvalType(s.Values[0])
				if tup, ok := val.(*typeTuple); ok {

					l := tup.all.Len()
					for i, v := range s.Names {
						if v.Name == "" || v.Name == "_" {
							continue
						}
						if i == l {
							break
						}
						t := newTypeVar(v.Name, tup.all.Index(i))
						r.nameds.Add(t)
					}
					continue loop
				}
			default:
				l := len(s.Values)
				for i, v := range s.Names {
					if v.Name == "" || v.Name == "_" {
						continue
					}
					if i == l {
						break
					}
					val := r.EvalType(s.Values[i])
					t := newTypeVar(v.Name, val)
					r.nameds.Add(t)
				}
				continue loop
			}
		}

		if val == nil {
			continue
		}
		for _, v := range s.Names {
			if v.Name == "" || v.Name == "_" {
				continue
			}

			t := newTypeVar(v.Name, val)
			r.nameds.Add(t)
		}
	}
}
