// Copyright 2026 Google LLC
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

package skilltoolset_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool/skilltoolset"
	"google.golang.org/adk/v2/tool/skilltoolset/skill"
)

type mockSource struct {
	skill.Source
	frontmatters []*skill.Frontmatter
}

func (m *mockSource) ListFrontmatters(ctx context.Context) ([]*skill.Frontmatter, error) {
	return m.frontmatters, nil
}

func TestProcessRequest(t *testing.T) {
	source := &mockSource{
		frontmatters: []*skill.Frontmatter{
			{
				Name:        "skill0",
				Description: "description0",
			},
			{
				Name:          "skill1",
				Description:   "description1",
				Compatibility: "...",
				Metadata: map[string]string{
					"flag1": "key1",
					"flag2": "key2",
				},
			},
		},
	}
	ts, err := skilltoolset.New(t.Context(), skilltoolset.Config{Source: source})
	if err != nil {
		t.Fatalf("skilltoolset.New failed: %v", err)
	}
	req := &model.LLMRequest{}

	err = ts.ProcessRequest(nil, req)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}
	if req.Config == nil {
		t.Fatalf("ProcessRequest: req.Config is nil")
	}
	if req.Config.SystemInstruction == nil {
		t.Fatalf("ProcessRequest: req.Config.SystemInstruction is nil")
	}
	if len(req.Config.SystemInstruction.Parts) != 1 {
		t.Fatalf("ProcessRequest: got %d parts, expected 1", len(req.Config.SystemInstruction.Parts))
	}
	gotText := req.Config.SystemInstruction.Parts[0].Text
	for _, want := range []string{"SKILL.md", "skills", "<available_skills>", "skill0", "description1"} {
		if !strings.Contains(gotText, want) {
			t.Errorf("ProcessRequest: got %q, want to contain %q", gotText, want)
		}
	}
}

// TestProcessRequest_LoadSkillInstructionUsesSchemaParamName guards that the
// default system instruction documents the load_skill tool with the parameter
// name its schema actually expects (`name`), not `skill_name`. The load_skill
// args struct (LoadSkillArgs.Name, json:"name") only accepts `name`, so an
// instruction telling the model to pass `skill_name` is rejected by ADK's
// input-schema validator on every first load (#912).
func TestProcessRequest_LoadSkillInstructionUsesSchemaParamName(t *testing.T) {
	source := &mockSource{
		frontmatters: []*skill.Frontmatter{{Name: "skill0", Description: "d"}},
	}
	ts, err := skilltoolset.New(t.Context(), skilltoolset.Config{Source: source})
	if err != nil {
		t.Fatalf("skilltoolset.New failed: %v", err)
	}
	req := &model.LLMRequest{}
	if err := ts.ProcessRequest(nil, req); err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}
	instr := req.Config.SystemInstruction.Parts[0].Text

	if !strings.Contains(instr, "`load_skill` tool with `name=\"<SKILL_NAME>\"`") {
		t.Errorf("default instruction should document load_skill with name=, got:\n%s", instr)
	}
	if strings.Contains(instr, "`load_skill` tool with `skill_name=") {
		t.Errorf("default instruction still documents load_skill with skill_name=, which its schema rejects:\n%s", instr)
	}
}

func TestProcessRequest_NoSkills(t *testing.T) {
	ts, err := skilltoolset.New(t.Context(), skilltoolset.Config{Source: &mockSource{}})
	if err != nil {
		t.Fatalf("skilltoolset.New failed: %v", err)
	}
	req := &model.LLMRequest{}

	err = ts.ProcessRequest(nil, req)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}
	if req.Config != nil && req.Config.SystemInstruction != nil && len(req.Config.SystemInstruction.Parts) > 0 {
		t.Errorf("ProcessRequest: got %d parts, expected 0", len(req.Config.SystemInstruction.Parts))
	}
}

func TestNew_MissingSource(t *testing.T) {
	_, err := skilltoolset.New(t.Context(), skilltoolset.Config{})
	if err == nil {
		t.Errorf("skilltoolset.New: expected error when source is missing")
	}
}

func TestTools(t *testing.T) {
	toolset, err := skilltoolset.New(t.Context(), skilltoolset.Config{Source: &mockSource{}})
	if err != nil {
		t.Fatalf("skilltoolset.New failed: %v", err)
	}

	tools, err := toolset.Tools(nil)
	if err != nil {
		t.Fatalf("Tools failed: %v", err)
	}
	var got []string
	for _, t := range tools {
		got = append(got, t.Name())
	}
	want := []string{"list_skills", "load_skill", "load_skill_resource"}
	if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("Tools result mismatch (-want +got):\n%s", diff)
	}
}
