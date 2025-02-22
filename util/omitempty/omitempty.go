// This is here until https://github.com/sqlc-dev/sqlc/pull/3117 is merged.
package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"
)

type FileMap map[string]FieldDecider

var filePaths = FileMap{
	"./pkg/database/models.go": AllFields{},
	"./pkg/database/calls.sql.go": FieldMap{
		"TalkerAlias": true,
		"Incidents":   true,
	},
}

type FieldDecider interface {
	Check(fields []*ast.Ident) bool
}

type FieldMap map[string]bool

func (fm FieldMap) Check(f []*ast.Ident) bool {
	for _, v := range f {
		if v != nil && fm[v.Name] {
			return true
		}
	}

	return false
}

type AllFields struct{}

func (AllFields) Check(_ []*ast.Ident) bool {
	return true
}

func main() {
	// Parse the source code
	for k, v := range filePaths {
		process(k, v)
	}
}

func process(filePath string, fd FieldDecider) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	// Modify the AST
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.StructType:
			for _, field := range x.Fields.List {
				if !fd.Check(field.Names) {
					continue
				}
				if field.Tag == nil {
					continue
				}
				if field.Tag.Value == "" || field.Tag.Kind != token.STRING {
					continue
				}

				field.Tag.Value = modifyJSONTag(field.Tag.Value)
			}
		}
		return true
	})

	// Write the output back to the original file
	var buf bytes.Buffer
	err = format.Node(&buf, fset, f)
	if err != nil {
		log.Fatal(err)
	}
	outputFile, err := os.Create(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer outputFile.Close()
	_, err = outputFile.Write(buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
}

func modifyJSONTag(tagValue string) string {
	tagValue = strings.Trim(tagValue, "`")

	tags := strings.Split(tagValue, " ")
	var modifiedTags []string

	for _, tag := range tags {
		// Only modify JSON tags, leave others as they are.
		if !strings.HasPrefix(tag, "json:") {
			modifiedTags = append(modifiedTags, tag)
			continue
		}

		jsonQuoted := tag[5:]                        // Remove "json:" prefix
		jsonValue := strings.Trim(jsonQuoted, "\"")  // Remove quotes
		jsonOptions := strings.Split(jsonValue, ",") // Split options

		// Check if "omitempty" is already present
		hasOmitempty := false
		for _, opt := range jsonOptions {
			if opt == "omitempty" {
				hasOmitempty = true
				break
			}
		}

		// Add "omitempty" if not present and the field is not ignored
		if !hasOmitempty && jsonOptions[0] != "-" {
			jsonOptions = append(jsonOptions, "omitempty")
		}

		// Reconstruct the JSON tag
		newJSONTag := "json:\"" + strings.Join(jsonOptions, ",") + "\""
		modifiedTags = append(modifiedTags, newJSONTag)
	}

	// Reconstruct the full tag
	return "`" + strings.Join(modifiedTags, " ") + "`"
}
