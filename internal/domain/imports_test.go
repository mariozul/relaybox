package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenImports lists packages that must never appear in the domain layer (RULE-ARCH-01/02).
var forbiddenImports = []string{
	"net/http",
	"database/sql",
	"github.com/jackc/pgx",
	"github.com/gin-gonic/gin",
	"github.com/labstack/echo",
	"google.golang.org/grpc",
	"github.com/go-chi/chi",
	"github.com/gorilla/mux",
}

func TestDomainImportPurity(t *testing.T) {
	t.Parallel()

	fs := token.NewFileSet()
	domainDir := "."
	files, err := os.ReadDir(domainDir)
	if err != nil {
		t.Fatalf("cannot read domain dir: %v", err)
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(domainDir, f.Name())
		node, err := parser.ParseFile(fs, filePath, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", f.Name(), err)
		}

		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range forbiddenImports {
				if strings.HasPrefix(importPath, forbidden) {
					t.Errorf("RULE-ARCH-01 violation: %s imports forbidden package %q", f.Name(), importPath)
				}
			}
		}
	}
}
