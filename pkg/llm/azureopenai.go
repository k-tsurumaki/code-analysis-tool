package llm

import (
	"context"
	"go/ast"
	"go/token"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

// AnalyzeFunctionAzureOpenAI はAzure EntraID認証でAzure OpenAIを使ってAI解析を行う関数です。
func AnalyzeFunctionAzureOpenAI(ctx context.Context, path string, fset *token.FileSet, fn *ast.FuncDecl, issues []Issue, task string) (*AIAnalysis, error) {
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	model := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	apiVersion := os.Getenv("AZURE_OPENAI_API_VERSION")
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	tenantID := os.Getenv("AZURE_TENANT_ID")
		
	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return nil, err
	}
	
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{"https://cognitiveservices.azure.com/.default"}})
	if err != nil {
		return nil, err
	}

	client, err := openai.New(
		openai.WithToken(token.Token),
		openai.WithModel(model),
		openai.WithBaseURL(endpoint),
		openai.WithAPIType(openai.APITypeAzureAD),
		openai.WithAPIVersion(apiVersion),
	)
	if err != nil {
		return nil, err
	}

	code := extractFuncSource(path, fset, fn)
	prompt, err := buildPromptFromFile(code, issues, task)
	if err != nil {
		return nil, err
	}

	out, err := llms.GenerateFromSinglePrompt(ctx, client, prompt, llms.WithTemperature(0.2), llms.WithMaxTokens(4096))
	if err != nil {
		return nil, err
	}

	return &AIAnalysis{Output: out}, nil
}
