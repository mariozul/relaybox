package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDomainImportPurity(t *testing.T) {
	t.Parallel()

	// RULE-ARCH-01: The domain layer must be pure — no HTTP frameworks,
	// database drivers, gRPC packages, or external third-party deps.
	forbidden := []string{
		"net/http",
		"database/sql",
		"github.com/jackc/pgx",
		"github.com/gin-gonic",
		"github.com/labstack",
		"github.com/gofiber",
		"github.com/gorilla",
		"google.golang.org/grpc",
		"github.com/go-chi",
	}

	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		if f == nil {
			return nil
		}

		for _, imp := range f.Imports {
			ip := strings.Trim(imp.Path.Value, "\"")
			for _, fb := range forbidden {
				if ip == fb || strings.HasPrefix(ip, fb+"/") {
					t.Errorf("RULE-ARCH-01: %s imports forbidden %q", path, ip)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
}
