package compiler

import (
	"strings"
	"testing"

	"github.com/klothoplatform/klotho/pkg/annotation"
	"github.com/klothoplatform/klotho/pkg/compiler/types"
	"github.com/klothoplatform/klotho/pkg/io"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/stretchr/testify/assert"
)

func TestCheckForProjectFile(t *testing.T) {
	tests := []struct {
		name      string
		fileUnits map[string]string
		fileName  string
		want      string
	}{
		{
			name: "no annotations returns empty string",
			fileUnits: map[string]string{
				"package.json": "",
			},
			fileName: "package.json",
			want:     "",
		},
		{
			name: "base test main exec unit no package json",
			fileUnits: map[string]string{
				"unitFile": `main`,
			},
			fileName: "package.json",
			want:     "",
		},
		{
			name: "base test main exec unit default match",
			fileUnits: map[string]string{
				"package.json": "",
				"unitFile":     `main`,
			},
			fileName: "package.json",
			want:     "package.json",
		},
		{
			name: "gets nested package json",
			fileUnits: map[string]string{
				"test/package.json": "",
				"test/unitFile":     `main`,
			},
			fileName: "package.json",
			want:     "test/package.json",
		},
		{
			name: "gets parent package json",
			fileUnits: map[string]string{
				"test/package.json": "",
				"test/one/unitFile": `main`,
			},
			fileName: "package.json",
			want:     "test/package.json",
		},
		{
			name: "gets root package json",
			fileUnits: map[string]string{
				"package.json":  "",
				"test/unitFile": `main`,
			},
			fileName: "package.json",
			want:     "package.json",
		},
		{
			name: "mismatch unit id returns empty",
			fileUnits: map[string]string{
				"package.json":  "",
				"test/unitFile": `not-main`,
			},
			fileName: "package.json",
			want:     "",
		},
		{
			name: "single exec unit annotations returns exactMatch",
			fileUnits: map[string]string{
				"package.json":      "",
				"test/package.json": "",
				"test/unitFile":     `main`,
			},
			fileName: "package.json",
			want:     "test/package.json",
		},
		{
			name: "multiple exec unit annotations returns exactMatch",
			fileUnits: map[string]string{
				"package.json":       "",
				"test/unitFile":      `main`,
				"other/unitFile":     `main`,
				"other/package.json": ``,
			},
			fileName: "package.json",
			want:     "other/package.json",
		},
		{
			name: "multiple exec unit annotations returns first exactMatch",
			fileUnits: map[string]string{
				"test/package.json":  "",
				"test/unitFile":      `main`,
				"other/unitFile":     `main`,
				"other/package.json": ``,
			},
			fileName: "package.json",
			want:     "test/package.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			testUnit := types.ExecutionUnit{Name: "main"}
			input := &types.InputFiles{}
			for path, unit := range tt.fileUnits {
				if strings.Contains(path, tt.fileName) {
					input.Add(&io.FileRef{FPath: path})
				} else {
					f, err := types.NewSourceFile(path, strings.NewReader(unit), testLang)
					if assert.Nil(err) {
						input.Add(f)
						testUnit.Add(f)
					}
				}
			}
			pf := types.CheckForProjectFile(input, &testUnit, tt.fileName)
			assert.Equal(tt.want, pf)
		})
	}
}

type testCapabilityFinder struct{}

var testLang = types.SourceLanguage{
	ID:               types.LanguageId("test_lang"),
	Sitter:           javascript.GetLanguage(), // we don't actually care about the language, but we do need a non-nil one
	CapabilityFinder: &testCapabilityFinder{},
}

func (t *testCapabilityFinder) FindAllCapabilities(sf *types.SourceFile) (types.AnnotationMap, error) {
	body := string(sf.Program())
	annots := types.AnnotationMap{
		types.AnnotationKey{Capability: annotation.ExecutionUnitCapability, ID: body}: &types.Annotation{
			Capability: &annotation.Capability{
				Name:       annotation.ExecutionUnitCapability,
				ID:         body,
				Directives: annotation.Directives{"id": body},
			},
		},
	}
	return annots, nil
}
