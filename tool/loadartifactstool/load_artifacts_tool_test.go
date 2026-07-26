// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadartifactstool_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/artifact"
	artifactinternal "google.golang.org/adk/v2/internal/artifact"
	icontext "google.golang.org/adk/v2/internal/context"
	"google.golang.org/adk/v2/internal/toolinternal"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool/loadartifactstool"
)

func TestLoadArtifactsTool_Run(t *testing.T) {
	loadArtifactsTool := loadartifactstool.New()
	tc := createToolContext(t)

	toolImpl, ok := loadArtifactsTool.(toolinternal.FunctionTool)
	if !ok {
		t.Fatal("loadArtifactsTool does not implement FunctionTool")
	}

	tests := []struct {
		name    string
		args    map[string]any
		want    map[string]any
		wantErr bool
	}{
		{
			name: "basic string slice",
			args: map[string]any{
				"artifact_names": []string{"file1", "file2"},
			},
			want: map[string]any{
				"artifact_names": []string{"file1", "file2"},
			},
		},
		{
			name: "empty args",
			args: map[string]any{},
			want: map[string]any{
				"artifact_names": []string{},
			},
		},
		{
			name: "any slice with strings",
			args: map[string]any{
				"artifact_names": []any{"fileA", "fileB"},
			},
			want: map[string]any{
				"artifact_names": []string{"fileA", "fileB"},
			},
		},
		{
			name: "empty string slice",
			args: map[string]any{
				"artifact_names": []string{},
			},
			want: map[string]any{
				"artifact_names": []string{},
			},
		},
		{
			name: "empty any slice",
			args: map[string]any{
				"artifact_names": []any{},
			},
			want: map[string]any{
				"artifact_names": []string{},
			},
		},
		{
			name: "nil value",
			args: map[string]any{
				"artifact_names": nil,
			},
			want: map[string]any{
				"artifact_names": []string{},
			},
		},
		{
			name: "incorrect type (not a slice)",
			args: map[string]any{
				"artifact_names": "not a slice",
			},
			wantErr: true,
		},
		{
			name: "any slice with non-string",
			args: map[string]any{
				"artifact_names": []any{"fileA", 123},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toolImpl.Run(tc, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if diff := cmp.Diff(tt.want, result); diff != "" {
				t.Errorf("Run() result diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadArtifactsTool_ProcessRequest(t *testing.T) {
	loadArtifactsTool := loadartifactstool.New()

	tc := createToolContext(t)
	artifacts := map[string]*genai.Part{
		"file1.txt": {Text: "content1"},
		"file2.pdf": {Text: "content2"},
	}
	for name, part := range artifacts {
		_, err := tc.Artifacts().Save(t.Context(), name, part)
		if err != nil {
			t.Fatalf("Failed to save artifact %s: %v", name, err)
		}
	}

	llmRequest := &model.LLMRequest{}

	requestProcessor, ok := loadArtifactsTool.(toolinternal.RequestProcessor)
	if !ok {
		t.Fatal("loadArtifactsTool does not implement RequestProcessor")
	}

	err := requestProcessor.ProcessRequest(tc, llmRequest)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}

	instruction := llmRequest.Config.SystemInstruction.Parts[0].Text
	if !strings.Contains(instruction, "You have a list of artifacts") {
		t.Errorf("Instruction should contain 'You have a list of artifacts', but got: %v", instruction)
	}
	if !strings.Contains(instruction, `"file1.txt"`) || !strings.Contains(instruction, `"file2.pdf"`) {
		t.Errorf("Instruction should contain artifact names, but got: %v", instruction)
	}
	if len(llmRequest.Contents) > 0 {
		t.Errorf("Expected no contents, but got: %v", llmRequest.Contents)
	}
}

func TestLoadArtifactsTool_ProcessRequest_Artifacts_LoadArtifactsFunctionCall(t *testing.T) {
	loadArtifactsTool := loadartifactstool.New()

	tc := createToolContext(t)
	artifacts := map[string]*genai.Part{
		"doc1.txt": {Text: "This is the content of doc1.txt"},
	}
	for name, part := range artifacts {
		_, err := tc.Artifacts().Save(t.Context(), name, part)
		if err != nil {
			t.Fatalf("Failed to save artifact %s: %v", name, err)
		}
	}

	functionResponse := &genai.FunctionResponse{
		Name: "load_artifacts",
		Response: map[string]any{
			"artifact_names": []string{"doc1.txt"},
		},
	}
	llmRequest := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: "model",
				Parts: []*genai.Part{
					genai.NewPartFromFunctionResponse(functionResponse.Name, functionResponse.Response),
				},
			},
		},
	}

	requestProcessor, ok := loadArtifactsTool.(toolinternal.RequestProcessor)
	if !ok {
		t.Fatal("loadArtifactsTool does not implement RequestProcessor")
	}

	err := requestProcessor.ProcessRequest(tc, llmRequest)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}

	if len(llmRequest.Contents) != 2 {
		t.Fatalf("Expected 2 content, but got: %v", llmRequest.Contents)
	}

	appendedContent := llmRequest.Contents[1]
	if appendedContent.Role != "user" {
		t.Errorf("Appended Content Role: got %v, want 'user'", appendedContent.Role)
	}
	if len(appendedContent.Parts) != 2 {
		t.Fatalf("Expected 2 parts in appended content, but got: %v", appendedContent.Parts)
	}
	if appendedContent.Parts[0].Text != "Artifact doc1.txt is:" {
		t.Errorf("First part of appended content: got %v, want 'Artifact doc1.txt is:'", appendedContent.Parts[0].Text)
	}
	if appendedContent.Parts[1].Text != "This is the content of doc1.txt" {
		t.Errorf("Second part of appended content: got %v, want 'This is the content of doc1.txt'", appendedContent.Parts[1].Text)
	}
}

// TestLoadArtifactsTool_ProcessRequest_MIMEHandling guards that loaded
// artifacts with inline MIME types Gemini does not accept inline are converted
// to model-safe parts instead of being forwarded as-is (which fails the next
// generateContent call with "Unsupported MIME Type"). Supported media passes
// through, text-like data is decoded to text, and other binary data becomes a
// descriptive placeholder.
func TestLoadArtifactsTool_ProcessRequest_MIMEHandling(t *testing.T) {
	tests := []struct {
		name       string
		artifact   *genai.Part
		wantInline bool   // expect the original inline part passed through unchanged
		wantText   string // otherwise expect a text part equal to this
	}{
		{
			name:       "supported image passes through inline",
			artifact:   &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte{0x89, 'P', 'N', 'G'}}},
			wantInline: true,
		},
		{
			name:     "csv inline data is decoded to text",
			artifact: &genai.Part{InlineData: &genai.Blob{MIMEType: "application/csv", Data: []byte("a,b\n1,2")}},
			wantText: "a,b\n1,2",
		},
		{
			name:     "unsupported binary becomes a placeholder",
			artifact: &genai.Part{InlineData: &genai.Blob{MIMEType: "application/vnd.ms-excel", Data: []byte{0x00, 0x01, 0x02, 0x03}}},
			wantText: `[Artifact "data" could not be inlined: unsupported MIME type "application/vnd.ms-excel" (4 bytes).]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadArtifactsTool := loadartifactstool.New()
			tc := createToolContext(t)
			const name = "data"
			if _, err := tc.Artifacts().Save(t.Context(), name, tt.artifact); err != nil {
				t.Fatalf("Save: %v", err)
			}

			req := &model.LLMRequest{
				Contents: []*genai.Content{{
					Role: "model",
					Parts: []*genai.Part{
						genai.NewPartFromFunctionResponse("load_artifacts", map[string]any{"artifact_names": []string{name}}),
					},
				}},
			}
			rp, ok := loadArtifactsTool.(toolinternal.RequestProcessor)
			if !ok {
				t.Fatal("loadArtifactsTool does not implement RequestProcessor")
			}
			if err := rp.ProcessRequest(tc, req); err != nil {
				t.Fatalf("ProcessRequest: %v", err)
			}

			if len(req.Contents) != 2 || len(req.Contents[1].Parts) != 2 {
				t.Fatalf("unexpected appended contents: %+v", req.Contents)
			}
			got := req.Contents[1].Parts[1]

			if tt.wantInline {
				if got.InlineData == nil || got.InlineData.MIMEType != tt.artifact.InlineData.MIMEType {
					t.Errorf("expected supported inline part to pass through, got %+v", got)
				}
				return
			}
			if got.InlineData != nil {
				t.Errorf("expected inline data to be converted away, but part is still inline: %+v", got.InlineData)
			}
			if got.Text != tt.wantText {
				t.Errorf("converted text = %q, want %q", got.Text, tt.wantText)
			}
		})
	}
}

func TestLoadArtifactsTool_ProcessRequest_Artifacts_OtherFunctionCall(t *testing.T) {
	loadArtifactsTool := loadartifactstool.New()

	tc := createToolContext(t)
	artifacts := map[string]*genai.Part{
		"doc1.txt": {Text: "content1"},
	}
	for name, part := range artifacts {
		_, err := tc.Artifacts().Save(t.Context(), name, part)
		if err != nil {
			t.Fatalf("Failed to save artifact %s: %v", name, err)
		}
	}

	functionResponse := &genai.FunctionResponse{
		Name: "other_function",
		Response: map[string]any{
			"some_key": "some_value",
		},
	}
	llmRequest := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: "model",
				Parts: []*genai.Part{
					genai.NewPartFromFunctionResponse(functionResponse.Name, functionResponse.Response),
				},
			},
		},
	}

	requestProcessor, ok := loadArtifactsTool.(toolinternal.RequestProcessor)
	if !ok {
		t.Fatal("loadArtifactsTool does not implement RequestProcessor")
	}

	err := requestProcessor.ProcessRequest(tc, llmRequest)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}
	if len(llmRequest.Contents) != 1 {
		t.Fatalf("Expected 1 content, but got: %v", llmRequest.Contents)
	}
	if llmRequest.Contents[0].Role != "model" {
		t.Errorf("Content Role: got %v, want 'model'", llmRequest.Contents[0].Role)
	}
}

func createToolContext(t *testing.T) agent.Context {
	t.Helper()

	artifacts := &artifactinternal.Artifacts{
		Service:   artifact.InMemoryService(),
		AppName:   "app",
		UserID:    "user",
		SessionID: "session",
	}

	ctx := icontext.NewInvocationContext(t.Context(), icontext.InvocationContextParams{
		Artifacts: artifacts,
	})

	return agent.NewToolContext(ctx, "", nil, nil)
}
