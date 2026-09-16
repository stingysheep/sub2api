//go:build unit

package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageModelSnapshotSurvivesGinContextReuse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request = request.WithContext(service.WithCompositeRouteDecision(request.Context(), service.CompositeRouteDecision{
		Matched: true, Source: service.CompositeRouteSourceExplicit, PublicModel: "first-public-model", TargetPlatform: service.PlatformOpenAI, UpstreamModel: "gpt-5",
	}))
	fields := clientRequestedUsageFields(c, service.ChannelMappingResult{MappedModel: "gpt-5"}, "gpt-5", "gpt-5")
	run := make(chan struct{})
	got := make(chan service.ChannelUsageFields, 1)
	go func() {
		<-run
		got <- fields
	}()
	c.Request = request.WithContext(service.WithCompositeRouteDecision(request.Context(), service.CompositeRouteDecision{
		Matched: true, Source: service.CompositeRouteSourceExplicit, PublicModel: "second-public-model", TargetPlatform: service.PlatformOpenAI, UpstreamModel: "gpt-5",
	}))
	close(run)
	result := <-got
	require.Equal(t, "first-public-model", result.OriginalModel)
	require.Equal(t, fields.ModelMappingChain, result.ModelMappingChain)
	require.Equal(t, "second-public-model", clientRequestedModel(c, "gpt-5"))
}

// All gateway entry points must snapshot request data before handing work to
// the usage pool: Gin may recycle the context before the callback runs.
func TestUsageWorkersDoNotCaptureGinContext(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !strings.HasPrefix(sel.Sel.Name, "submit") || !strings.Contains(sel.Sel.Name, "UsageRecordTask") {
				return true
			}
			for _, arg := range call.Args {
				worker, ok := arg.(*ast.FuncLit)
				if !ok {
					continue
				}
				checked++
				ast.Inspect(worker.Body, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok && id.Name == "c" {
						t.Errorf("%s: usage worker captures original Gin context", fset.Position(id.Pos()))
					}
					return true
				})
			}
			return true
		})
	}
	require.GreaterOrEqual(t, checked, 12)
}
