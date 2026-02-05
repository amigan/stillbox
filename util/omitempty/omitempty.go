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

	"dynatron.me/x/stillbox/internal/common"
)

type FileMap map[string]FieldDecider

type FieldTag int

const (
	NotSet    FieldTag = 0
	OmitEmpty FieldTag = 1 << iota
	OmitZero
	GenerateYaml
)

var filePaths = FileMap{
	"./pkg/database/models.go": AllFields(OmitEmpty | GenerateYaml),
	"./pkg/database/calls.sql.go": FieldMap{
		"TalkerAlias":                OmitEmpty,
		"Incidents":                  OmitEmpty | OmitZero,
		"HasTranscript":              OmitZero,
		"ListCallsPRow:Source":       OmitZero,
		"ListCallsPRow:Transcript":   OmitEmpty,
		"ListCallsPRow:MissingAudio": OmitZero,
	},
}

type FieldDecider interface {
	Check(typeName string, fields []*ast.Ident) FieldTag
}

type FieldMap map[string]FieldTag

func (fm FieldMap) Check(typeName string, f []*ast.Ident) FieldTag {
	for _, v := range f {
		if v != nil && fm[typeName+":"+v.Name] != NotSet {
			return fm[typeName+":"+v.Name]
		}
		if v != nil && fm[v.Name] != NotSet {
			return fm[v.Name]
		}
	}

	return NotSet
}

type AllFields FieldTag

func (a AllFields) Check(_ string, _ []*ast.Ident) FieldTag {
	return FieldTag(a)
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
	var lastTypeName string
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			lastTypeName = x.Name.Name
		case *ast.StructType:
			for _, field := range x.Fields.List {
				res := fd.Check(lastTypeName, field.Names)
				if res == NotSet {
					continue
				}
				if field.Tag == nil {
					continue
				}
				if field.Tag.Value == "" || field.Tag.Kind != token.STRING {
					continue
				}

				field.Tag.Value = modifyTag(res, field.Tag.Value)
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
	defer outputFile.Close() //nolint:errcheck
	_, err = outputFile.Write(buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
}

func modifyTag(res FieldTag, tagValue string) string {
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
		curTags := NotSet
		for _, opt := range jsonOptions {
			switch opt {
			case "omitempty":
				curTags |= OmitEmpty
			case "omitzero":
				curTags |= OmitZero
			}
		}

		// Add field if not present and the field is not ignored
		if jsonOptions[0] != "-" {
			if res&OmitEmpty > 0 && curTags&OmitEmpty == 0 {
				jsonOptions = append(jsonOptions, "omitempty")
			}

			if res&OmitZero > 0 && curTags&OmitZero == 0 {
				jsonOptions = append(jsonOptions, "omitzero")
			}

		}

		// Reconstruct the JSON tag
		newJSONTag := "json:\"" + strings.Join(jsonOptions, ",") + "\""
		modifiedTags = append(modifiedTags, newJSONTag)

		if res&GenerateYaml > 0 {
			jsonOptions[0] = common.ToSnake(jsonOptions[0])
			yamlTag := `yaml:"` + strings.Join(jsonOptions, ",") + `"`
			modifiedTags = append(modifiedTags, yamlTag)
		}
	}

	// Reconstruct the full tag
	return "`" + strings.Join(modifiedTags, " ") + "`"
}
